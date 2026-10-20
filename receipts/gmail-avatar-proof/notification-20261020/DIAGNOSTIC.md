# Sender-photo completion diagnostic — 2026-10-20

Owner: CompleteSenderPhotoNotification. Scope: retained Qt sender-photo completion and its actual InboxView consumer; no pipeline/auth UI rewrite.

Initial evidence (before editing or testing):
- `git diff -- qt` inspected the retained diff (10 tracked Qt paths, including archive/row-continuation hunks that remain out of scope).
- `stat /tmp/lb-avatar-qt-build receipts/gmail-avatar-proof/newrun-20261020-1302/PROOF.md` succeeded (exit 0).
- The retained proof explicitly records that online paint required `store.refresh()` after cache completion (§3.5); offline reopen painted the same cached bytes. This is the baseline failure, not a new reproduction or real-Google grant proof.
- `CardStore::applyRemoteSenderPhoto` writes `m_cards[row].sender_photo`, emits unqualified `dataChanged`, and emits `senderPhotoChanged`.
- A retained dedicated `SenderPhotoRole` exists, but InboxView still copies only `source.card`, `source.cardId`, and `source.title` into its presentation snapshot. The three avatar consumers read the nested copied card map rather than the dedicated role. The completion-to-presentation boundary is the narrow repair target.
- `qt/tests/tst_senderphotopaint.qml` and the valid fixture wiring were retained, not discarded. No test has been run mid-edit.

Acceptance remains: first-online actual-consumer paint without manual refresh, offline cache reopen, default/missing fallbacks, authorization controls, malformed-image negative control, frozen out-of-tree build, private Xvfb/HOME/D-Bus, durable exact-path/hash/command receipt. Real Google enablement, owner consent, and release gates remain explicit and unproved. No shared installer, production account, Google grant, user desktop/session bus/phone/TV, Git mutation or metadata mutation is authorized.
