The current behavior is renovate is unable to install golang, resulting in these errors:

```
 WARN: artifactErrors (repository=tpo/anti-censorship/rdsys, branch=renovate/github.com-xanzy-go-gitlab-0.x)
       "artifactErrors": [
         {
           "lockFile": "go.sum",
           "stderr": "Command failed: install-tool golang 1.21.9\n"
         }
       ]
```

The debug logs are:

```
DEBUG: rawExec err (repository=tpo/anti-censorship/rdsys, branch=renovate/github.com-prometheus-client_golang-1.x)
       "err": {
         "cmd": "/bin/sh -c install-tool golang 1.21.10",
         "stderr": "",
         "stdout": "installing v2 tool golang v1.21.10\n[12:44:14.482] INFO (476): Installing tool golang@1.21.10...\nlinking tool golang v1.21.10\ngo: downloading go1.22.2 (linux/amd64)\ngo: download go1.22.2: golang.org/toolchain@v0.0.1-go1.22.2.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off\n[12:44:22.858] FATAL (476): Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\n    err: {\n      \"type\": \"Error\",\n      \"message\": \"Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"stack\":\n          Error: Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\n              at makeError (/snapshot/dist/containerbase-cli.js:40199:13)\n              at handlePromise (/snapshot/dist/containerbase-cli.js:40914:29)\n              at process.processTicksAndRejections (node:internal/process/task_queues:95:5)\n              at async InstallLegacyToolService.execute (/snapshot/dist/containerbase-cli.js:52974:5)\n              at async InstallToolService.execute (/snapshot/dist/containerbase-cli.js:53158:9)\n              at async InstallToolShortCommand.execute (/snapshot/dist/containerbase-cli.js:53368:14)\n              at async InstallToolShortCommand.validateAndExecute (/snapshot/dist/containerbase-cli.js:2430:26)\n              at async _Cli.run (/snapshot/dist/containerbase-cli.js:3543:22)\n              at async _Cli.runExit (/snapshot/dist/containerbase-cli.js:3551:28)\n              at async main (/snapshot/dist/containerbase-cli.js:53562:3)\n      \"shortMessage\": \"Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"command\": \"/usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"escapedCommand\": \"\\\"/usr/local/containerbase/bin/install-tool.sh\\\" golang 1.21.10\",\n      \"exitCode\": 1,\n      \"cwd\": \"/builds/tpo/tpa/renovate-cron/renovate/repos/gitlab/tpo/anti-censorship/rdsys\",\n      \"failed\": true,\n      \"timedOut\": false,\n      \"isCanceled\": false,\n      \"killed\": false\n    }\n[12:44:23.598] INFO (476): Installed tool golang with errors in 9.1s.\n",
         "options": {
           "cwd": "/builds/tpo/tpa/renovate-cron/renovate/repos/gitlab/tpo/anti-censorship/rdsys",
           "encoding": "utf-8",
           "env": {
             "GOPATH": "/go",
             "GOSUMDB": "off",
             "GOFLAGS": "-modcacherw",
             "GIT_CONFIG_KEY_0": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_0": "ssh://**redacted**@github.com/",
             "GIT_CONFIG_KEY_1": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_1": "git@github.com:",
             "GIT_CONFIG_KEY_2": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_2": "https://github.com/",
             "GIT_CONFIG_COUNT": "6",
             "GIT_CONFIG_KEY_3": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_3": "ssh://**redacted**@gitlab.torproject.org/",
             "GIT_CONFIG_KEY_4": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_4": "git@gitlab.torproject.org:",
             "GIT_CONFIG_KEY_5": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_5": "https://gitlab.torproject.org/",
             "HOME": "/home/ubuntu",
             "PATH": "/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
             "LC_ALL": "C.UTF-8",
             "LANG": "C.UTF-8",
             "CONTAINERBASE_CACHE_DIR": "/builds/tpo/tpa/renovate-cron/renovate/cache/containerbase"
           },
           "maxBuffer": 10485760,
           "timeout": 900000
         },
         "exitCode": 1,
         "name": "ExecError",
         "message": "Command failed: install-tool golang 1.21.10\n",
         "stack": "ExecError: Command failed: install-tool golang 1.21.10\n\n    at ChildProcess.<anonymous> (/usr/local/renovate/lib/util/exec/common.ts:99:11)\n    at ChildProcess.emit (node:events:529:35)\n    at ChildProcess.emit (node:domain:489:12)\n    at Process.ChildProcess._handle.onexit (node:internal/child_process:292:12)"
       },
       "durationMs": 9357
DEBUG: Failed to update go.sum (repository=tpo/anti-censorship/rdsys, branch=renovate/github.com-prometheus-client_golang-1.x)
       "err": {
         "cmd": "/bin/sh -c install-tool golang 1.21.10",
         "stderr": "",
         "stdout": "installing v2 tool golang v1.21.10\n[12:44:14.482] INFO (476): Installing tool golang@1.21.10...\nlinking tool golang v1.21.10\ngo: downloading go1.22.2 (linux/amd64)\ngo: download go1.22.2: golang.org/toolchain@v0.0.1-go1.22.2.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off\n[12:44:22.858] FATAL (476): Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\n    err: {\n      \"type\": \"Error\",\n      \"message\": \"Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"stack\":\n          Error: Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\n              at makeError (/snapshot/dist/containerbase-cli.js:40199:13)\n              at handlePromise (/snapshot/dist/containerbase-cli.js:40914:29)\n              at process.processTicksAndRejections (node:internal/process/task_queues:95:5)\n              at async InstallLegacyToolService.execute (/snapshot/dist/containerbase-cli.js:52974:5)\n              at async InstallToolService.execute (/snapshot/dist/containerbase-cli.js:53158:9)\n              at async InstallToolShortCommand.execute (/snapshot/dist/containerbase-cli.js:53368:14)\n              at async InstallToolShortCommand.validateAndExecute (/snapshot/dist/containerbase-cli.js:2430:26)\n              at async _Cli.run (/snapshot/dist/containerbase-cli.js:3543:22)\n              at async _Cli.runExit (/snapshot/dist/containerbase-cli.js:3551:28)\n              at async main (/snapshot/dist/containerbase-cli.js:53562:3)\n      \"shortMessage\": \"Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"command\": \"/usr/local/containerbase/bin/install-tool.sh golang 1.21.10\",\n      \"escapedCommand\": \"\\\"/usr/local/containerbase/bin/install-tool.sh\\\" golang 1.21.10\",\n      \"exitCode\": 1,\n      \"cwd\": \"/builds/tpo/tpa/renovate-cron/renovate/repos/gitlab/tpo/anti-censorship/rdsys\",\n      \"failed\": true,\n      \"timedOut\": false,\n      \"isCanceled\": false,\n      \"killed\": false\n    }\n[12:44:23.598] INFO (476): Installed tool golang with errors in 9.1s.\n",
         "options": {
           "cwd": "/builds/tpo/tpa/renovate-cron/renovate/repos/gitlab/tpo/anti-censorship/rdsys",
           "encoding": "utf-8",
           "env": {
             "GOPATH": "/go",
             "GOSUMDB": "off",
             "GOFLAGS": "-modcacherw",
             "GIT_CONFIG_KEY_0": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_0": "ssh://**redacted**@github.com/",
             "GIT_CONFIG_KEY_1": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_1": "git@github.com:",
             "GIT_CONFIG_KEY_2": "url.https://**redacted**@github.com/.insteadOf",
             "GIT_CONFIG_VALUE_2": "https://github.com/",
             "GIT_CONFIG_COUNT": "6",
             "GIT_CONFIG_KEY_3": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_3": "ssh://**redacted**@gitlab.torproject.org/",
             "GIT_CONFIG_KEY_4": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_4": "git@gitlab.torproject.org:",
             "GIT_CONFIG_KEY_5": "url.https://**redacted**@gitlab.torproject.org/.insteadOf",
             "GIT_CONFIG_VALUE_5": "https://gitlab.torproject.org/",
             "HOME": "/home/ubuntu",
             "PATH": "/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
             "LC_ALL": "C.UTF-8",
             "LANG": "C.UTF-8",
             "CONTAINERBASE_CACHE_DIR": "/builds/tpo/tpa/renovate-cron/renovate/cache/containerbase"
           },
           "maxBuffer": 10485760,
           "timeout": 900000
         },
         "exitCode": 1,
         "name": "ExecError",
         "message": "Command failed: install-tool golang 1.21.10\n",
         "stack": "ExecError: Command failed: install-tool golang 1.21.10\n\n    at ChildProcess.<anonymous> (/usr/local/renovate/lib/util/exec/common.ts:99:11)\n    at ChildProcess.emit (node:events:529:35)\n    at ChildProcess.emit (node:domain:489:12)\n    at Process.ChildProcess._handle.onexit (node:internal/child_process:292:12)"
       }

```

If I use the container that renovate is using and try to do the install command:
```
$podman run -it -v /home/micah/dev/tor/rdsys:/tmp/rdsys ghcr.io/renovatebot/renovate:37.353.0@sha256:1e9801c491fa802867b7307d0675e343e3d32fd9bcc13321a91836311e289710 /bin/bash
ubuntu@191831612c3b:/usr/src/app$ cd /tmp/rdsys/
ubuntu@191831612c3b:/tmp/rdsys$ GOPATH=/go
ubuntu@191831612c3b:/tmp/rdsys$ GOSUMDB=off
ubuntu@191831612c3b:/tmp/rdsys$ GOFLAGS='-modcacherw'
ubuntu@191831612c3b:/tmp/rdsys$ HOME='/home/ubuntu'
ubuntu@191831612c3b:/tmp/rdsys$ PATH='/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
ubuntu@191831612c3b:/tmp/rdsys$ LC_ALL='C.UTF-8'
ubuntu@191831612c3b:/tmp/rdsys$ LANG='C.UTF-8'
ubuntu@191831612c3b:/tmp/rdsys$ install-tool golang 1.21.10
[21:03:40.977] INFO (6): Installing tool golang@1.21.10...
installing v2 tool golang v1.21.10
linking tool golang v1.21.10
go: downloading go1.22.2 (linux/amd64)
go: download go1.22.2: golang.org/toolchain@v0.0.1-go1.22.2.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off
[21:03:59.080] FATAL (6): Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10
    err: {
      "type": "Error",
      "message": "Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10",
      "stack":
          Error: Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10
              at makeError (/snapshot/dist/containerbase-cli.js:40199:13)
              at handlePromise (/snapshot/dist/containerbase-cli.js:40914:29)
              at process.processTicksAndRejections (node:internal/process/task_queues:95:5)
              at async InstallLegacyToolService.execute (/snapshot/dist/containerbase-cli.js:52974:5)
              at async InstallToolService.execute (/snapshot/dist/containerbase-cli.js:53158:9)
              at async InstallToolShortCommand.execute (/snapshot/dist/containerbase-cli.js:53368:14)
              at async InstallToolShortCommand.validateAndExecute (/snapshot/dist/containerbase-cli.js:2430:26)
              at async _Cli.run (/snapshot/dist/containerbase-cli.js:3543:22)
              at async _Cli.runExit (/snapshot/dist/containerbase-cli.js:3551:28)
              at async main (/snapshot/dist/containerbase-cli.js:53562:3)
      "shortMessage": "Command failed with exit code 1: /usr/local/containerbase/bin/install-tool.sh golang 1.21.10",
      "command": "/usr/local/containerbase/bin/install-tool.sh golang 1.21.10",
      "escapedCommand": "\"/usr/local/containerbase/bin/install-tool.sh\" golang 1.21.10",
      "exitCode": 1,
      "cwd": "/tmp/rdsys",
      "failed": true,
      "timedOut": false,
      "isCanceled": false,
      "killed": false
    }
[21:04:00.059] INFO (6): Installed tool golang with errors in 19s.
ubuntu@191831612c3b:/tmp/rdsys$ 
```

Notice that I'm installing v1.21.10, and it seems to do so, but then for some reason, it is proceeding to install v1.22.2, and that is where it is failing:

```
linking tool golang v1.21.10
go: downloading go1.22.2 (linux/amd64)
go: download go1.22.2: golang.org/toolchain@v0.0.1-go1.22.2.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off
```

I don't know why it decides to install a different version after the first.

If I do the same thing, but I do not set GOSUBDB=off, then it compiles, without problems:

```
$ podman run -it -v /home/micah/dev/tor/rdsys:/tmp/rdsys ghcr.io/renovatebot/renovate:37.353.0@sha256:1e9801c491fa802867b7307d0675e343e3d32fd9bcc13321a91836311e289710 /bin/bash
ubuntu@a6bc5573bbf3:/usr/src/app$ 
ubuntu@a6bc5573bbf3:/usr/src/app$ 
ubuntu@a6bc5573bbf3:/usr/src/app$ GOPATH=/go
ubuntu@a6bc5573bbf3:/usr/src/app$ GOFLAGS='-modcacherw'
ubuntu@a6bc5573bbf3:/usr/src/app$ HOME='/home/ubuntu'
ubuntu@a6bc5573bbf3:/usr/src/app$ PATH='/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/.cargo/bin:/home/ubuntu/.local/bin:/go/bin:/home/ubuntu/bin:/home/ubuntu/.npm-global/bin:/home/ubuntu/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
ubuntu@a6bc5573bbf3:/usr/src/app$ LC_ALL='C.UTF-8'
ubuntu@a6bc5573bbf3:/usr/src/app$ LANG='C.UTF-8'
ubuntu@a6bc5573bbf3:/usr/src/app$ install-tool golang 1.21.10
installing v2 tool golang v1.21.10
[21:19:53.805] INFO (6): Installing tool golang@1.21.10...
linking tool golang v1.21.10
go version go1.21.10 linux/amd64
GO111MODULE=''
GOARCH='amd64'
GOBIN=''
GOCACHE='/home/ubuntu/.cache/go-build'
GOENV='/home/ubuntu/.config/go/env'
GOEXE=''
GOEXPERIMENT=''
GOFLAGS=''
GOHOSTARCH='amd64'
GOHOSTOS='linux'
GOINSECURE=''
GOMODCACHE='/go/pkg/mod'
GONOPROXY=''
GONOSUMDB=''
GOOS='linux'
GOPATH='/go'
GOPRIVATE=''
GOPROXY='https://proxy.golang.org,direct'
GOROOT='/opt/containerbase/tools/golang/1.21.10'
GOSUMDB='off'
GOTMPDIR=''
GOTOOLCHAIN='auto'
GOTOOLDIR='/opt/containerbase/tools/golang/1.21.10/pkg/tool/linux_amd64'
GOVCS=''
GOVERSION='go1.21.10'
GCCGO='gccgo'
GOAMD64='v1'
AR='ar'
CC='gcc'
CXX='g++'
CGO_ENABLED='0'
GOMOD='/dev/null'
GOWORK=''
CGO_CFLAGS='-O2 -g'
CGO_CPPFLAGS=''
CGO_CXXFLAGS='-O2 -g'
CGO_FFLAGS='-O2 -g'
CGO_LDFLAGS='-O2 -g'
PKG_CONFIG='pkg-config'
GOGCCFLAGS='-fPIC -m64 -Wl,--no-gc-sections -fmessage-length=0 -ffile-prefix-map=/tmp/go-build2094397949=/tmp/go-build -gno-record-gcc-switches'
[21:20:01.326] INFO (6): Installed tool golang in 7.5s.
````
