# nova

A DPI bypass manager for Windows built around `winws.exe` from [bol-van/zapret](https://github.com/bol-van/zapret).

nova keeps strategies as TOML files, builds the winws command line from them, runs winws in a console or as a Windows service, finds the strategy that works for your ISP and diagnoses common conflicts.

## Install

1. Add the folder to your antivirus exclusions before unpacking. WinDivert is often flagged as `Not-a-virus:RiskTool.Multi.WinDivert`.
   ```powershell
   Add-MpPreference -ExclusionPath "C:\nova"
   ```
2. Unpack the release archive to `C:\` so the files end up in `C:\nova`.
   ```powershell
   Expand-Archive nova-<version>.zip -DestinationPath C:\
   cd C:\nova
   Get-ChildItem -Recurse | Unblock-File
   ```
3. Run everything below from a terminal started as administrator.

## Usage

```
nova probe --install            test all strategies and install the best one as a service
nova diag [--fix]               find conflicts and environment problems
nova list                       list strategies
nova show [strategy]            print the winws command line
nova run [strategy]             run winws in this console, Ctrl+C to stop
nova service install [strategy] install and start the service
nova service remove             remove the service and the WinDivert driver
nova service status
nova config                     print settings
nova set ipset loaded           none | any | loaded
nova set game_filter all        off | tcp | udp | all
```

Add your own domains to `lists/list-general-user.txt` and exclusions to `lists/list-exclude-user.txt` and `lists/ipset-exclude-user.txt`. Empty lists are not passed to winws.

## How probe works

nova first checks the targets without any bypass. Then, for each strategy, it starts winws in the background, sends HEAD requests to every target over HTTP/1.1, TLS 1.2 and TLS 1.3, and stops winws. The strategy with the most successful checks wins.

The default targets are Discord, YouTube, Google and Cloudflare. Put a `targets.toml` next to `nova.exe` to override them.

`--freeze` adds a check for the "TCP 16-20 KB" block on hosting providers: nova uploads 64 KB and reports a freeze when the upload stalls without a response. `--freeze-host` adds your own HTTPS host to that check.

winws logs and reports are written to `logs/`. The nova service is paused while probe runs.

## Layout

```
nova.exe
nova.toml       settings
bin/            winws.exe, cygwin1.dll, WinDivert, fake packets, SHA256SUMS
lists/          host and IP lists
strategies/     *.toml
```

## Development

```
make sync     # pull bin/, lists/ and strategies from flowseal/zapret-discord-youtube
make test     # tests and go vet for windows
make dist     # dist/nova-<version>.zip
```

`internal/batimport` converts upstream `general*.bat` files into `strategies/*.toml`. `TestParity` checks that the command line built for every strategy matches the upstream one.

## Credits

- [bol-van/zapret](https://github.com/bol-van/zapret): winws.exe, MIT
- [basil00/WinDivert](https://github.com/basil00/WinDivert): packet capture driver, LGPL-3.0 / GPL-2.0
- [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube): strategies, lists and fake packets, MIT
- [hyperion-cs/dpi-checkers](https://github.com/hyperion-cs/dpi-checkers): the TCP 16-20 test suite, Apache-2.0. `internal/probe/suite.json` is a fallback copy of [suite.v2.json](https://hyperion-cs.github.io/dpi-checkers/ru/tcp-16-20/suite.v2.json)
