package middleware

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	apiKeyRoutingBodyMemoryBytes       = 64 << 10
	apiKeyRoutingJSONBufferBytes       = 32 << 10
	apiKeyRoutingStringMaxBytes        = 4 << 10
	apiKeyRoutingJSONMaxDepth          = 256
	apiKeyRoutingMaxActiveScans        = 64
	apiKeyRoutingMaxActiveScansPerUser = 4
	// One default 256 MiB gateway body plus MaxBytesReader's over-read margin.
	apiKeyRoutingSpoolBudgetBytes = 257 << 20
	apiKeyRoutingReplayContextKey = "api_key_routing_body_replay"
)

var (
	errInvalidAPIKeyRoutingJSON = errors.New("invalid JSON while scanning request model")
	errAPIKeyRoutingSpoolLimit  = errors.New("API key routing spool capacity exhausted")
	errAPIKeyGroupRoutingBusy   = infraerrors.ServiceUnavailable(
		"API_KEY_GROUP_ROUTING_BUSY",
		"API key group routing is temporarily busy",
	)
	apiKeyRoutingScanSlots        = make(chan struct{}, apiKeyRoutingMaxActiveScans)
	apiKeyRoutingActiveSpoolBytes atomic.Int64
	apiKeyRoutingScanUsersMu      sync.Mutex
	apiKeyRoutingActiveUserScans  = make(map[int64]int)
)

func resolveAPIKeyRequestGroup(c *gin.Context, apiKey *service.APIKey) (*service.APIKey, error) {
	if platform, ok := GetForcePlatformFromContext(c); ok && strings.TrimSpace(platform) != "" {
		return resolveAPIKeyRequestPlatformOrUnavailableDefault(apiKey, platform)
	}
	if !apiKeyHasGroupBindings(apiKey) {
		return apiKey, nil
	}
	model, modelErr := requestModelForAPIKeyRoutingWithError(c.Request, apiKey.User.ID)
	if replay, ok := c.Request.Body.(*replayedRequestBody); ok {
		c.Set(apiKeyRoutingReplayContextKey, replay)
	}
	if modelErr != nil {
		return nil, modelErr
	}
	return resolveAPIKeyRequestPlatformOrUnavailableDefault(apiKey, service.APIKeyRequestPlatformForModel(model))
}

func resolveGoogleAPIKeyRequestGroup(c *gin.Context, apiKey *service.APIKey) (*service.APIKey, error) {
	if forced, ok := GetForcePlatformFromContext(c); ok && strings.TrimSpace(forced) != "" {
		return resolveAPIKeyRequestPlatformOrUnavailableDefault(apiKey, forced)
	}
	if !apiKeyHasGroupBindings(apiKey) {
		return apiKey, nil
	}
	return resolveAPIKeyRequestPlatformOrUnavailableDefault(apiKey, service.PlatformGemini)
}

func resolveAPIKeyRequestPlatformOrUnavailableDefault(apiKey *service.APIKey, platform string) (*service.APIKey, error) {
	selected, err := service.ResolveAPIKeyRequestPlatform(apiKey, platform)
	if err == nil {
		return selected, nil
	}
	if unavailableDefaultGroupMatchesPlatform(apiKey, platform) {
		return apiKey, nil
	}
	if unavailableGroup, ok := uniqueUnavailableBoundGroupForPlatform(apiKey, platform); ok {
		return apiKey.WithSelectedGroup(unavailableGroup.ID)
	}
	return nil, err
}

func uniqueUnavailableBoundGroupForPlatform(apiKey *service.APIKey, platform string) (*service.Group, bool) {
	if apiKey == nil || strings.TrimSpace(platform) == "" {
		return nil, false
	}
	var match *service.Group
	for i := range apiKey.Groups {
		group := &apiKey.Groups[i]
		if group.ID <= 0 || group.IsActive() || !strings.EqualFold(group.Platform, strings.TrimSpace(platform)) {
			continue
		}
		if match != nil {
			return nil, false
		}
		match = group
	}
	return match, match != nil
}

func unavailableDefaultGroupMatchesPlatform(apiKey *service.APIKey, platform string) bool {
	if apiKey == nil || apiKey.GroupID == nil {
		return false
	}
	if apiKey.Group == nil {
		return strings.TrimSpace(platform) == ""
	}
	if apiKey.Group.ID != *apiKey.GroupID || apiKey.Group.IsActive() {
		return false
	}
	return strings.TrimSpace(platform) == "" || strings.EqualFold(apiKey.Group.Platform, strings.TrimSpace(platform))
}

func apiKeyHasGroupBindings(apiKey *service.APIKey) bool {
	return apiKey != nil && (apiKey.GroupID != nil || apiKey.Group != nil || len(apiKey.GroupIDs) > 0 || len(apiKey.Groups) > 0)
}

func requestModelForAPIKeyRouting(req *http.Request) string {
	model, _ := requestModelForAPIKeyRoutingWithError(req, 0)
	return model
}

func requestModelForAPIKeyRoutingWithError(req *http.Request, userID int64) (string, error) {
	if req == nil || req.Body == nil || !requestMethodCanCarryModel(req.Method) || !requestContentTypeCanCarryJSON(req.Header.Get("Content-Type")) {
		return "", nil
	}
	releaseSlot, acquired := tryAcquireAPIKeyRoutingScanSlot(userID)
	if !acquired {
		return "", errAPIKeyGroupRoutingBusy
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			releaseSlot()
		}
	}()

	original := req.Body
	captured := &apiKeyRoutingBodyCapture{}
	model, err := scanTopLevelRequestModel(io.TeeReader(original, captured))
	replay := captured.replay(original)
	req.Body = replay
	if replay.tempFile != nil {
		replay.releaseScanSlot = releaseSlot
		releaseOnReturn = false
	}
	if captured.writeErr != nil {
		return "", errAPIKeyGroupRoutingBusy
	}
	if err != nil {
		return "", nil
	}
	return model, nil
}

func tryAcquireAPIKeyRoutingScanSlot(userID int64) (func(), bool) {
	apiKeyRoutingScanUsersMu.Lock()
	if apiKeyRoutingActiveUserScans[userID] >= apiKeyRoutingMaxActiveScansPerUser {
		apiKeyRoutingScanUsersMu.Unlock()
		return nil, false
	}
	apiKeyRoutingActiveUserScans[userID]++
	apiKeyRoutingScanUsersMu.Unlock()

	select {
	case apiKeyRoutingScanSlots <- struct{}{}:
	default:
		releaseAPIKeyRoutingUserScan(userID)
		return nil, false
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			<-apiKeyRoutingScanSlots
			releaseAPIKeyRoutingUserScan(userID)
		})
	}, true
}

func releaseAPIKeyRoutingUserScan(userID int64) {
	apiKeyRoutingScanUsersMu.Lock()
	defer apiKeyRoutingScanUsersMu.Unlock()
	active := apiKeyRoutingActiveUserScans[userID]
	if active <= 1 {
		delete(apiKeyRoutingActiveUserScans, userID)
		return
	}
	apiKeyRoutingActiveUserScans[userID] = active - 1
}

func scanTopLevelRequestModel(source io.Reader) (string, error) {
	reader := bufio.NewReaderSize(source, apiKeyRoutingJSONBufferBytes)
	opening, err := readNextRoutingJSONByte(reader)
	if err != nil {
		return "", err
	}
	if opening != '{' {
		return "", nil
	}

	for {
		next, err := readNextRoutingJSONByte(reader)
		if err != nil {
			return "", err
		}
		if next == '}' {
			return "", nil
		}
		if next != '"' {
			return "", errInvalidAPIKeyRoutingJSON
		}
		key, err := readRoutingJSONString(reader)
		if err != nil {
			return "", err
		}
		colon, err := readNextRoutingJSONByte(reader)
		if err != nil || colon != ':' {
			return "", errInvalidAPIKeyRoutingJSON
		}
		valueStart, err := readNextRoutingJSONByte(reader)
		if err != nil {
			return "", err
		}
		if key == "model" && valueStart == '"' {
			model, err := readRoutingJSONString(reader)
			if err != nil {
				return "", err
			}
			if model = strings.TrimSpace(model); model != "" {
				return model, nil
			}
		} else if err := skipRoutingJSONValue(reader, valueStart); err != nil {
			return "", err
		}

		separator, err := readNextRoutingJSONByte(reader)
		if err != nil {
			return "", err
		}
		switch separator {
		case ',':
			continue
		case '}':
			return "", nil
		default:
			return "", errInvalidAPIKeyRoutingJSON
		}
	}
}

func readNextRoutingJSONByte(reader *bufio.Reader) (byte, error) {
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		switch value {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return value, nil
		}
	}
}

func readRoutingJSONString(reader *bufio.Reader) (string, error) {
	raw := make([]byte, 0, 64)
	raw = append(raw, '"')
	escaped := false
	tooLong := false
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if !tooLong {
			raw = append(raw, value)
			tooLong = len(raw) > apiKeyRoutingStringMaxBytes+2
		}
		if escaped {
			escaped = false
			continue
		}
		if value == '\\' {
			escaped = true
			continue
		}
		if value != '"' {
			continue
		}
		if tooLong {
			return "", errInvalidAPIKeyRoutingJSON
		}
		unquoted, err := strconv.Unquote(string(raw))
		if err != nil {
			return "", errInvalidAPIKeyRoutingJSON
		}
		return unquoted, nil
	}
}

func skipRoutingJSONValue(reader *bufio.Reader, first byte) error {
	switch first {
	case '"':
		return skipRoutingJSONString(reader)
	case '{', '[':
		stack := []byte{first}
		for len(stack) > 0 {
			value, err := reader.ReadByte()
			if err != nil {
				return err
			}
			switch value {
			case '"':
				if err := skipRoutingJSONString(reader); err != nil {
					return err
				}
			case '{', '[':
				if len(stack) >= apiKeyRoutingJSONMaxDepth {
					return errInvalidAPIKeyRoutingJSON
				}
				stack = append(stack, value)
			case '}', ']':
				opening := stack[len(stack)-1]
				if (opening == '{' && value != '}') || (opening == '[' && value != ']') {
					return errInvalidAPIKeyRoutingJSON
				}
				stack = stack[:len(stack)-1]
			}
		}
		return nil
	default:
		for {
			value, err := reader.ReadByte()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			switch value {
			case ',', '}', ']':
				return reader.UnreadByte()
			case ' ', '\t', '\n', '\r':
				return nil
			}
		}
	}
}

func skipRoutingJSONString(reader *bufio.Reader) error {
	escaped := false
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if escaped {
			escaped = false
			continue
		}
		if value == '\\' {
			escaped = true
			continue
		}
		if value == '"' {
			return nil
		}
	}
}

func requestContentTypeCanCarryJSON(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func requestMethodCanCarryModel(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

type apiKeyRoutingBodyCapture struct {
	memory        bytes.Buffer
	tempFile      *os.File
	tempPath      string
	tail          bytes.Buffer
	reservedBytes int64
	writeErr      error
}

func (c *apiKeyRoutingBodyCapture) Write(p []byte) (int, error) {
	if c.tempFile == nil && c.memory.Len()+len(p) <= apiKeyRoutingBodyMemoryBytes {
		return c.memory.Write(p)
	}
	if c.tempFile == nil {
		if err := c.spillToTempFile(); err != nil {
			_, _ = c.memory.Write(p)
			c.writeErr = err
			return len(p), err
		}
	}
	requested := int64(len(p))
	if !reserveAPIKeyRoutingSpoolBytes(requested) {
		_, _ = c.tail.Write(p)
		c.writeErr = errAPIKeyRoutingSpoolLimit
		return len(p), c.writeErr
	}
	n, err := c.tempFile.Write(p)
	c.reservedBytes += int64(n)
	releaseAPIKeyRoutingSpoolBytes(requested - int64(n))
	if n < len(p) {
		_, _ = c.tail.Write(p[n:])
		if err == nil {
			err = io.ErrShortWrite
		}
	}
	if err != nil {
		c.writeErr = err
		return len(p), err
	}
	return len(p), nil
}

func (c *apiKeyRoutingBodyCapture) spillToTempFile() error {
	initialBytes := int64(c.memory.Len())
	if !reserveAPIKeyRoutingSpoolBytes(initialBytes) {
		return errAPIKeyRoutingSpoolLimit
	}
	reserved := true
	defer func() {
		if reserved {
			releaseAPIKeyRoutingSpoolBytes(initialBytes)
		}
	}()
	file, err := os.CreateTemp("", "sub2api-api-key-routing-*")
	if err != nil {
		return err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	written, err := io.Copy(file, bytes.NewReader(c.memory.Bytes()))
	if err != nil || written != initialBytes {
		cleanup()
		if err != nil {
			return err
		}
		return io.ErrShortWrite
	}
	reserved = false
	c.memory.Reset()
	c.tempFile = file
	c.tempPath = file.Name()
	c.reservedBytes = initialBytes
	return nil
}

func reserveAPIKeyRoutingSpoolBytes(amount int64) bool {
	if amount <= 0 {
		return true
	}
	for {
		current := apiKeyRoutingActiveSpoolBytes.Load()
		if current > apiKeyRoutingSpoolBudgetBytes-amount {
			return false
		}
		if apiKeyRoutingActiveSpoolBytes.CompareAndSwap(current, current+amount) {
			return true
		}
	}
}

func releaseAPIKeyRoutingSpoolBytes(amount int64) {
	if amount > 0 {
		apiKeyRoutingActiveSpoolBytes.Add(-amount)
	}
}

func (c *apiKeyRoutingBodyCapture) replay(original io.ReadCloser) *replayedRequestBody {
	body := &replayedRequestBody{
		original:      original,
		tempFile:      c.tempFile,
		tempPath:      c.tempPath,
		memoryBytes:   c.memory.Len() + c.tail.Len(),
		reservedBytes: c.reservedBytes,
	}
	readers := make([]io.Reader, 0, 3)
	if c.tempFile != nil {
		_, _ = c.tempFile.Seek(0, io.SeekStart)
		readers = append(readers, &apiKeyRoutingTempReader{body: body})
	}
	if c.memory.Len() > 0 {
		readers = append(readers, bytes.NewReader(c.memory.Bytes()))
	}
	if c.tail.Len() > 0 {
		readers = append(readers, bytes.NewReader(c.tail.Bytes()))
	}
	readers = append(readers, original)
	body.Reader = io.MultiReader(readers...)
	return body
}

type replayedRequestBody struct {
	io.Reader
	original        io.Closer
	tempFile        *os.File
	tempPath        string
	memoryBytes     int
	reservedBytes   int64
	releaseScanSlot func()
	tempOnce        sync.Once
	closeOnce       sync.Once
	closeErr        error
}

func (b *replayedRequestBody) Close() error {
	b.closeOnce.Do(func() {
		b.cleanupTempFile()
		if b.original != nil {
			b.closeErr = errors.Join(b.closeErr, b.original.Close())
		}
	})
	return b.closeErr
}

func (b *replayedRequestBody) cleanupTempFile() {
	b.tempOnce.Do(func() {
		if b.tempFile != nil {
			b.closeErr = errors.Join(b.closeErr, b.tempFile.Close())
		}
		if b.tempPath != "" {
			if err := os.Remove(b.tempPath); err != nil && !os.IsNotExist(err) {
				b.closeErr = errors.Join(b.closeErr, err)
			}
		}
		releaseAPIKeyRoutingSpoolBytes(b.reservedBytes)
		if b.releaseScanSlot != nil {
			b.releaseScanSlot()
		}
	})
}

type apiKeyRoutingTempReader struct {
	body *replayedRequestBody
}

func cleanupAPIKeyRoutingRequestBody(c *gin.Context) {
	if c == nil {
		return
	}
	if value, ok := c.Get(apiKeyRoutingReplayContextKey); ok {
		if replay, ok := value.(*replayedRequestBody); ok && replay != nil {
			_ = replay.Close()
			return
		}
	}
	if replay, ok := c.Request.Body.(*replayedRequestBody); ok && replay != nil {
		_ = replay.Close()
	}
}

func (r *apiKeyRoutingTempReader) Read(p []byte) (int, error) {
	if r.body == nil || r.body.tempFile == nil {
		return 0, io.EOF
	}
	n, err := r.body.tempFile.Read(p)
	if err == io.EOF {
		r.body.cleanupTempFile()
	}
	return n, err
}

func abortWithAPIKeyGroupRoutingError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	if status < http.StatusBadRequest {
		status = http.StatusInternalServerError
	}
	reason := strings.TrimSpace(infraerrors.Reason(err))
	if reason == "" {
		reason = "API_KEY_GROUP_ROUTING_FAILED"
	}
	message := strings.TrimSpace(infraerrors.Message(err))
	if message == "" {
		message = "Failed to select an API key group for this request"
	}
	AbortWithError(c, status, reason, message)
}

func abortWithGoogleAPIKeyGroupRoutingError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	if status < http.StatusBadRequest {
		status = http.StatusInternalServerError
	}
	message := strings.TrimSpace(infraerrors.Message(err))
	if message == "" {
		message = "Failed to select an API key group for this request"
	}
	abortWithGoogleError(c, status, message)
}
