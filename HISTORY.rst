*******
HISTORY
*******

3.2.7 (2024-03-12)
==================

- NO functional change. Only a rebuild with Golang 1.22.1 to avoid some
  vulnerability with crypto/x509 https://pkg.go.dev/vuln/GO-2024-2598
  and with net/textproto https://pkg.go.dev/vuln/GO-2024-2599

3.2.6 (2024-03-01)
==================

- config file now contains a beaverversion entry. If this entry is not left blank, beaver will try
to match its version with the desired version. If the two versions do not match beaver will refuse
to process the resources to avoid getting a different results.


3.2.5 (2024-03-01)
==================

- no functional change. Just bumped the golang release from 1.21 to 1.22.0 to
  avoid potential vulnerability in the filepath.join functions on windows


3.2.4 (2024-03-01)
==================

 - no feature added


3.2.3 (2023-10-18)
==================

- added more tests. No feature added for the user


3.2.2 (2023-10-17)
==================

- disabled keyword in charts can now be a variable

3.2.1 (2023-10-17)
==================

- namespace beaver can now be a variable

3.2.0 (2023-10-17)
==================

- build namespace flag to force helm chart namespace
- bump ci to 1.20.6 to avoid vulncheck
- Fix logging: the wrong logger was used in 'build'
- [ytt] reverse the layer paths order on the command line
- ns name in rs name if ns exists in rs
- beaver now hydrate filename of rs
- recursive variable hydration support added

3.1.9 (2023-03-23)
==================

- can now send output to stdout instead of directory wuth `-o stdout`
- can now force the namespace on a Helm chart
- upgraded to quay.orus.io/cloudcrane/golang:1.20.2-bullseye
- upgraded golangci-lint to 1.51.2

3.1.8 (2022-12-06)
==================

- fix a bug in the hydrate function that caused expressions with multiple
  variables in the same line to fail to render properly. Exemple:
  <[image]>:<[tag]> failed to render with a cryptic error

3.1.7 (2022-11-25)
==================

- fix a nasty bug in the hydate function that caused some documents containing
  comment lines composed of multiple dashes to be improperly interpreted as
  multiple yaml docs. This cause a wrong ouput. You should upgrade to this
  release to avoid encountering this issue.

3.1.6 (2022-11-05)
==================

- skipped 3.1.5 because it was on golang 1.19.2 and contained a vulnerability
  upgraded to golang 1.19.3. See https://pkg.go.dev/vuln/GO-2022-1095 for more
  info.
- multiple inherit support for beaver files
- can now disable a chart
- support to rename a chart in the beaver definition with the `name` key
  allowing to use `-` in produced names
- hydrate function now allows for non string variables in beaver variables
- move to go1.19+ to fix some CVEs
- added govulncheck to our ci toolchain

3.1.4 (2022-09-15)
==================

WARNING: this version does not change anything functionally BUT the output is
now always "properly" indented (as per our yaml lib) and this may change your
output even if no source is changed on your side. We greatly advise to run a
beaver build on your beaver project with NO other change than the beaver
version and review the results.

- tooling now has a `task vulncheck` which tries and find golang vulnerabilities
  for our project
- updated yaml dependency to gopkg.in/yaml.v3 after discovering vulnerabilities
  in the yaml.v2 lib we used

3.1.3 (2022-09-14)
==================

- dry run: fix a nil pointer exception due to the dry run returning nil
  as an openfile
