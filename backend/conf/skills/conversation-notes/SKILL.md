---
name: Conversation Notes
description: Append notes to a file that persists in the current conversation workspace.
---

# Conversation Notes

Use `notes.py` to append a note to `notes.txt` in the current conversation workspace.

Repeated calls in the same conversation reuse the same file. Different conversations have separate files.

## Parameters

Pass a JSON object containing `note`.

## Output

The script returns all notes currently stored for the conversation.
