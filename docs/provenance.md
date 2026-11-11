# Source provenance

The existing production repository is unchanged. This repository uses a new root commit, excludes prebuilt historical archives and records source lineage explicitly instead of copying large binary history.

| Source | Fixed revision | Imported material |
|---|---|---|
| chinahch/php-fpm-core | c8058a2f14060d8d2d12adaaef218ff558a1df98 | install-v2, tests, XHTTP patch/test, audit overlay, audit tools/docs; legacy workflows/scripts preserved under docs/legacy |
| chinahch/php-fpm-core | 2a5ae50eed9bb400995601293a4ebd484920b0fd | Native per-kernel IP-family Python block extracted verbatim from build workflow into scripts/apply_ipfamily.py; patches/strict_ipfamily.py |
| cedar2025/Xboard-Node | 0a29338e1f102a462363ce3527417029f89bab28 | Not vendored: fetched at build time, verified by commit ID and exported with git archive |

New work: atomic prepared-source staging; source.lock.json; build manifest with patch SHA256 values; normalized packaging; explicit checksum verification; isolated CI; documentation and regression tests.

Patch order: XHTTP/TLS + Machine kernel translation -> native per-kernel IP policy -> strict opposite-family guard -> CLI path compatibility/disable upstream self-upgrade -> optional destination-audit overlay -> gofmt.

Preparation invokes preserved Python patches in UTF-8 mode. Historical Go 1.26.3 compilation hit an internal compiler error in an AWS Route53 dependency on Windows; verified builds use Go 1.26.10, matching the historical dual-IP build toolchain choice.
