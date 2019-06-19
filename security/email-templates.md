# etcd Security Process Email Templates

This is a collection of email templates to handle various situations the security team encounters.

## Upcoming security release

```
Subject: Upcoming security release of etcd $VERSION
To: etcd-dev@googlegroups.com
Cc: security@etcd-io

Hello etcd Community,

The etcd Product Security Committee and maintainers would like to announce the forthcoming release
of etcd $VERSION.

This release will be made available on the $ORDINALDAY of $MONTH $YEAR at
$PDTHOUR PDT ($GMTHOUR GMT). This release will fix $NUMDEFECTS security
defect(s). The highest rated security defect is considered $SEVERITY severity.

No further details or patches will be made available in advance of the release.

**Thanks**

Thanks to $REPORTER, $DEVELOPERS, and the $RELEASELEADS for the coordination is making this release.

Thanks,

$PERSON on behalf of the etcd Product Security Committee and maintainers
```

## Security Fix Announcement

```
Subject: Security release of etcd $VERSION is now available
To: etcd-dev@googlegroups.com
Cc: security@etcd-io

Hello etcd Community,

The Product Security Committee and maintainers would like to announce the availability of etcd $VERSION.
This addresses the following CVE(s):

* CVE-YEAR-ABCDEF (CVSS score $CVSS): $CVESUMMARY
...

Upgrading to $VERSION is encouraged to fix these issues.

**Am I vulnerable?**

Run `etcd --version` and if it indicates a base version of $OLDVERSION or
older that means it is a vulnerable version.

<!-- Provide details on features, extensions, configuration that make it likely that a system is
vulnerable in practice. -->

**How do I mitigate the vulnerability?**

<!--
[This is an optional section. Remove if there are no mitigations.]
-->

**How do I upgrade?**

Follow the upgrade instructions at https://etcd.io/docs

**Vulnerability Details**

<!--
[For each CVE]
-->

***CVE-YEAR-ABCDEF***

$CVESUMMARY

This issue is filed as $CVE. We have rated it as [$CVSSSTRING]($CVSSURL)
($CVSS, $SEVERITY) [See the GitHub issue for more details]($GITHUBISSUEURL)

**Thanks**

Thanks to $REPORTER, $DEVELOPERS, and the $RELEASELEADS for the
coordination in making this release.

Thanks,

$PERSON on behalf of the etcd Product Security Committee and maintainers
```
