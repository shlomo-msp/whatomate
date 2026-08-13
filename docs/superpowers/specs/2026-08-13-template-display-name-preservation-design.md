# Template Display Name Preservation Design

## Goal

Keep Whatomate's friendly template display name independent from the technical
Meta template name throughout editing and synchronization.

## Behavior

For an existing local template, Meta synchronization refreshes provider-owned
fields but preserves `display_name`. A template imported from Meta for the
first time continues to initialize `display_name` from Meta's technical name.

Saving only a different `display_name` is a local metadata update. It keeps an
`APPROVED` or `REJECTED` template's current status because the change is not
submitted to Meta. Any change to language, category, header, body, footer,
buttons, samples, or authentication-template settings retains the existing
transition to `DRAFT`.

## Compatibility

The API request and response shapes remain unchanged. Template publishing and
sending still use `name`; Reservations reconciliation continues matching on
account, name, and local template ID.

## Verification

Handler regression tests cover display-only saves, Meta-owned content edits,
preservation during an existing-template sync, and name fallback for newly
imported Meta templates.
