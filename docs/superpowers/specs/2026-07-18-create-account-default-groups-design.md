# Create Account Default Groups Design

## Goal

When the create-account dialog opens, select every group compatible with the current account platform by default.

## Behavior

- Select only groups visible under the existing `GroupSelector` platform rules.
- Recompute the default selection when the user changes platform or Antigravity mixed-scheduling scope.
- Support groups supplied asynchronously after the dialog opens.
- Once the user manually changes the group selection, later group-list refreshes must preserve that choice.
- Closing the dialog resets the touched state so the next new account starts with defaults again.
- Existing accounts and persisted group assignments are unaffected.

## Implementation

Keep the behavior inside `CreateAccountModal.vue`. A compatibility helper mirrors `GroupSelector` filtering without search state. A per-dialog touched flag distinguishes defaults from user input. The selector update event marks the field touched; initialization watchers update only untouched selections.

## Testing

Component tests verify initial selection, platform compatibility, asynchronous group loading, and preservation of manual deselection.
