
## v0.2.0

#### breaking changes
- move entry metadata components into Protobuf definition (#119)
- simplify Page/PageKeys definition in Entry (#125)

#### features
- improve searcher, verifier, and storer concurrency performance (#139, #140)
- add Document storage Prometheus metrics (#136, #137 and Grafana dashboard (#146)
- improve local and cloud cluster operation tooling (#133, #143)
- publish Docker `snapshot` image off develop (#127)
- define k8s container resource limits (#142)
- add more build info to librarian startup banner (#149)

#### bugfixes
- server can now listen on non-localhost (#126)
- prevent `id.(Short)Hex` panic when value shorter than expected (#129)
- fix race conditions (#130, #131)

#### internal
- standardizing error panics (#120, #121, #123)
- move server-only storage code from `common/storage` back into `server/storage` (#135)
- standardize Put/Get timeout config across acceptance test (#141)
- bump dependency versions (#148)
- simplify linter config and fix lint issues (#151)


## v0.1.0

Initial public release with most planned functionality.
