## Generate new pb.go files

 1. Update
[DATA_PLANE_API_VERSION](https://github.com/grpc/grpc-go/blob/master/xds/internal/regenerate_scripts/envoy-proto-gen.sh#L3)
to a new commit for
[envoyproxy/data-plane-api](https://github.com/envoyproxy/data-plane-api/commits/master).
 1. Run `./envoy-proto-gen.sh` in __this__ directory (and fingers crossed).

## It didn't work

 - If applying patch failed, look at the patches, manually apply them, and
 generate new patches.
 - It might help to see the new changes. To see github diff between two commits
 https://github.com/envoyproxy/data-plane-api/compare/sha1..sha2
   - E.g.
   https://github.com/envoyproxy/data-plane-api/compare/965c278c10fa90ff34cb4d4890141863f4437b4a..2bcd30b5f4c224a98d1394f4266335ac640f178d

## It worked, diff we __don't__ want

 1. `go.mod` shouldn't be changed
    - Look for `github.com/envoyproxy/protoc-gen-validate/validate`, it should
    __NOT__ be added. Because it depends on gogo proto, and we have a local
    validate library to replace it.
