# Post-1.0 physical-device bounds checklist

This checklist belongs to the separately approved post-1.0 native client. It
is recorded now so the desktop proxy cannot silently become a mobile claim.

- Test at least one supported low-memory Android phone, one representative
  Android phone, and every supported iOS memory class; record model, OS,
  storage state, thermal state, battery mode, ABI, and build.
- Repeat the v0.8 100/10,000-operation, hostile-admission, pending-queue,
  108 MB resource-resume, and 64 MiB pack cases through the shipping FFI and
  storage adapters.
- Measure cold/warm wall time, CPU time, peak PSS/RSS, storage growth, file
  count, open descriptors, energy, cancellation latency, and foreground UI
  responsiveness. Include background suspension and process-kill recovery.
- Exercise slow/lossy networks, HTTP range restart, nearly-full storage,
  interrupted atomic admission, wrong keys/passwords, corrupt carriers, and a
  peer beyond retention that must snapshot-resync.
- Lower limits for the weakest supported device or negotiate an explicitly
  compatible lower receive limit. Never raise a signed protocol limit from one
  high-end-device result.
- Publish aggregate-only evidence and the final supported-device matrix before
  claiming physical mobile support.
