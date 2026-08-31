# v0.7 G20 release acceptance

G20 closes v0.7 by re-running the supported-system gates and binding their
results to one machine-checked release matrix. The evidence in this directory
does not claim a second read of any private corpus. It revalidates the frozen,
aggregate-only G14e/G17 profiles and combines them with current production
tests for peer convergence, both carriers, disaster recovery, lazy resources,
conflicts, retention, abuse resistance, documentation, upgrades, and packaging.

Run the focused closure gate with:

```bash
make g20-validate
```

The final release command remains `scripts/package_release.sh`; it runs the
full repository and documentation checks before producing and verifying the
source ZIP. `UPGRADE_ROLLBACK.md` records the supported v18-to-v27 upgrade and
the snapshot-based rollback procedure. `DEPENDENCY_LICENSES.json` inventories
all exact Go modules and every package in every committed npm lockfile; its
offline validator fails closed on drift or an unapproved license.

No G20 command pushes, tags, publishes, writes a reserve, or burns media.
