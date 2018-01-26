
## v0.3.0

#### breaking changes
- libri client balancers now require a random number generator in their constructors (#154)
- cluster deploy script now uses consistent nomenclature for its cluster directory (#155)
- use compressed instead of uncompressed public keys (#167) 

#### bugfixes
- fix race condition in search and verify operations (#153)
- fix peer slice order in peer distance heap (#165)
- fix external logging package (#171)

#### external features
- add author `ShareEnvelope` method for creating and uploading a new from an existing envelope (#152)
- add librarian profiler endpoint (#157)
- bump libri Docker container image to Alpine Linux 3.7 and install a few other helpful bash tools (#161)
- pool connections to (other) librarians instead of always re-creating them (#163, #164)
- improve peer ordering within a routing table bucket (#166)
- update Grafana dashboards (#168)
- export more error types (#169)
- add ecid.ID save to/load from file (#170)

#### internal features
- remove now-unnecessary filtered logging from acceptance test (#155)
- author now deletes internally-saved pages after writing contents to output (#158)
- author now uses in-memory DB instead of RocksDB for transient page storage (#159)
- librarian RocksDB options optimized (#160)
- set librarian server grpc max concurrent streams for lower-variance transport performance (#162)
- add release script (#172)


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
