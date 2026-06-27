# Copyright 2024 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:alpine AS build
RUN apk add --no-cache git make
WORKDIR /build
COPY . .
ENV CGO_ENABLED=0
RUN make build

FROM alpine:latest
COPY --from=build /build/dist/yamlfmt /bin/yamlfmt
WORKDIR /project
ENTRYPOINT ["/bin/yamlfmt"]
