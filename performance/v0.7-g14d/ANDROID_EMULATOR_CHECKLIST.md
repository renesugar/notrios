# Android-emulator follow-up checklist

G14d's Go core and desktop REST proxy satisfy the pre-mobile bounded contract.
No Android emulator or physical mobile device was run in this slice, and the
desktop peak-RSS result is not presented as a mobile result.

When the v0.8 Flutter/Go client owns an Android runtime, run this checklist on
the pinned emulator image before enabling physical catch-up there:

- record emulator image, API level, ABI, RAM, storage filesystem, and free
  space before/after each phase;
- request an explicitly authorized physical snapshot through the desktop
  proxy and interrupt/resume downloads below 1 MiB, at a frame boundary, above
  2 GiB, and at the final byte;
- verify wrong key, signature, schema/capability, truncation, and tamper are
  refused before canonical replacement;
- inject termination at every durable restore stage and confirm normal startup
  remains blocked until the same restore rolls forward;
- verify the emergency snapshot before cutover and after recovery;
- confirm the installed replica has a fresh ID, correct vector/floors, retained
  unavailable-resource declarations, and converges after incremental replay;
- measure phase wall/CPU time, Java/native/Go peak memory, apparent/allocated
  bytes, file count, maximum directory entries, and foreground responsiveness;
- repeat through directory/removable-media transport and REST using identical
  sealed bytes; and
- keep physical-device thermal/background/storage-provider validation as a
  separate post-emulator gate.
