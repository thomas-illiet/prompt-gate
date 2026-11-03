Local security patch of deepmerge 4.3.1 (MIT). Upstream: https://github.com/TehShrike/deepmerge.

GHSA-gcqx-w48m-w2h4 has no upstream patched release. This copy rejects __proto__, constructor and prototype in both target and source enumeration, including null-prototype targets. It retains the upstream merge, array and customMerge behavior. Remove this override once upstream publishes a fixed release, after running dependency-security.test.ts and the full frontend checks.
