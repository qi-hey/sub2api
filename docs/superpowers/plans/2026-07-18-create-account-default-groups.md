# Create Account Default Groups Implementation Plan

> **For AI agent:** Execute with TDD and verify each step before deployment.

**Goal:** Default every new account to all groups compatible with its selected platform without overriding later manual choices.

**Architecture:** Add local default-selection state and compatibility filtering to the existing create-account modal. Keep `GroupSelector` presentational and preserve its existing filtering contract.

**Tech stack:** Vue 3, TypeScript, Vue Test Utils, Vitest.

---

### Task 1: Add failing component coverage

**Files:**
- Modify: `frontend/src/components/account/__tests__/CreateAccountModal.spec.ts`

- [ ] Mount the modal in normal mode with Anthropic and OpenAI groups.
- [ ] Assert the selector receives all current-platform group IDs.
- [ ] Assert switching platform replaces defaults with compatible IDs.
- [ ] Assert an asynchronous groups update initializes an untouched form.
- [ ] Assert manual deselection remains after a later groups refresh.
- [ ] Run the focused test and confirm it fails because `group_ids` is empty.

### Task 2: Implement default compatible-group selection

**Files:**
- Modify: `frontend/src/components/account/CreateAccountModal.vue`

- [ ] Replace direct `v-model` binding with an explicit update handler that marks the field touched.
- [ ] Add a compatibility helper matching `GroupSelector` platform and mixed-scheduling rules.
- [ ] Initialize untouched selections on dialog open, platform changes, mixed-scheduling changes, and groups updates.
- [ ] Reset the touched state when the dialog closes.
- [ ] Run focused tests and confirm they pass.

### Task 3: Verify and deploy

**Files:**
- Verify: frontend test suite and production build.

- [ ] Run `pnpm.cmd run test:run`.
- [ ] Run `pnpm.cmd run build`.
- [ ] Build the embedded Linux binary with version `0.1.158-custom-anyrouter`.
- [ ] Push the custom branch, main, and refreshed custom tag.
- [ ] Back up the VPS database, environment, service unit, and binary.
- [ ] Deploy with one service restart and verify health, version, checksum, and existing account scheduling configuration.
