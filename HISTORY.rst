*******
HISTORY
*******

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
