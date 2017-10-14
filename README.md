# OCI CAS-Protocol Go implementations

This repository implements:

* The [CAS-Engine Protocols][registry] in [`registry.go`](registry.go).
* A generic interface used by the registry in [`interface.go`](interface.go).
* The [OCI CAS Template Protocol][oci-cas-template-v1] in [`template`](template).

There are command-line bindings in [`oci-cas`](cmd/oci-cas), which reads a CAS-engine configurations from [stdin][], resolves digests given as arguments, and writes their verified content to [stdout][stdin].

```
$ cat cas-engines.json
[
  {
    "config": {
      "protocol": "oci-cas-template-v1",
      "uri": "cas/{algorithm}/{encoded}"
    },
    "uri": "https://example.com"
  }
]
$ oci-cas sha256:c98c24b677eff44860afea6f493bbaec5bb1c4cbb209c6fc2bbb47f66ff2ad31 <cas-engines.json
Hello, World!
```

For more information, see `oci-cas --help`.

[casEngines]: https://github.com/xiekeyang/oci-discovery/blob/0be7eae246ae9a975a76ca209c045043f0793572/xdg-ref-engine-discovery.md#ref-engines-objects
[oci-cas-template-v1]: https://github.com/xiekeyang/oci-discovery/blob/0be7eae246ae9a975a76ca209c045043f0793572/cas-template.md
[registry]: https://github.com/xiekeyang/oci-discovery/blob/0be7eae246ae9a975a76ca209c045043f0793572/cas-engine-protocols.md
[stdin]: http://pubs.opengroup.org/onlinepubs/9699919799/functions/stdin.html
