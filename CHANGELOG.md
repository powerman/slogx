# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.2] - 2026-11-19

### 🐛 Fixed

- Stop built-in layout attrs from leaking into the next record by @powerman in [1f0c503]
- Apply layout keys to built-in time and level attrs by @powerman in [71fe97c]
- Output only the last attr for layout keys including built-in ones by @powerman in [d730bca]
- Avoid panic when WrapHandler next handler is uncomparable by @powerman in [6c2d0c1]
- Do not retain large layout attr buffers in the pool by @powerman in [72e17e7]
- Use default context for context.TODO in NewDefaultContextLogger by @powerman in [e0882fe]
- Separate attrs after With attr with empty formatted output by @powerman in [38ff7e5]

### ⚡ Performance

- Avoid allocations for layout keys by @powerman in [586e007]
- Avoid allocations in LayoutHandler WithAttrs and WithGroup by @powerman in [f0f6269]
- Format custom time layouts without allocation in LayoutHandler by @powerman in [607cfa7]
- Keep LayoutHandler key prefix buffer in pooled handle state by @powerman in [d9cf8d5]
- Insert LayoutHandler prefix attrs without a second buffer by @powerman in [e18486b]
- Format Level values without allocation by @powerman in [d9a940a]
- Output formatted built-in attrs without generic attr processing by @powerman in [547a2cd]
- Format source position without fmt by @powerman in [add073f]
- Share LayoutHandler layoutAttrs with handlers made by WithGroup by @powerman in [5e77c56]
- Format LayoutHandler source attr without generic attr processing by @powerman in [74b18b5]

### 📚 Documentation

- Describe vendored files and LayoutHandler divergence from upstream by @powerman in [b63358d]
- Describe logging recovered panics with Stack by @powerman in [2fc1e18]
- Correct frames shown by Stack in recovered panic logs by @powerman in [b52cb45]
- Fix stale comments in LayoutHandler by @powerman in [a4e6285]
- Fix misleading comments in LayoutHandler handleState by @powerman in [604942f]
- Update LayoutHandler file header with copyright and origin by @powerman in [6cd8193]

[0.5.2]: https://github.com/powerman/slogx/compare/v0.5.1..v0.5.2
[1f0c503]: https://github.com/powerman/slogx/commit/1f0c50304a40a44e1e64fd7fd85db39a6ab0d840
[71fe97c]: https://github.com/powerman/slogx/commit/71fe97cf596c2fb5fa1b21d829694f25e3a89cfd
[d730bca]: https://github.com/powerman/slogx/commit/d730bcac413aa0e7c4969983500e4720ed238b51
[586e007]: https://github.com/powerman/slogx/commit/586e007beb4c141d86040d52559783ac315dc2f5
[b63358d]: https://github.com/powerman/slogx/commit/b63358df33375bfc91a1a2b9c3ca426de76568d0
[6c2d0c1]: https://github.com/powerman/slogx/commit/6c2d0c19b83ad073214c512670f507ab273236e2
[72e17e7]: https://github.com/powerman/slogx/commit/72e17e7cdd5b6139790765cac165a817abcf5aa2
[e0882fe]: https://github.com/powerman/slogx/commit/e0882fe496a352fd4039642ffd60eb69c9ef3201
[f0f6269]: https://github.com/powerman/slogx/commit/f0f62696c73bb279d7afab3be0b1397dd0580f88
[2fc1e18]: https://github.com/powerman/slogx/commit/2fc1e18acbe470426ac3d7b0aa947e608c2c57e5
[b52cb45]: https://github.com/powerman/slogx/commit/b52cb4522721741f57b372015fcae1ee71a96bb6
[607cfa7]: https://github.com/powerman/slogx/commit/607cfa7da3ee2750230bf803fcf7f2b9a3a09282
[d9cf8d5]: https://github.com/powerman/slogx/commit/d9cf8d5ab3e127de6d6e3e7d80de6569560e02c6
[a4e6285]: https://github.com/powerman/slogx/commit/a4e6285fd8cdec6650f056b0e136650496c7b9ed
[38ff7e5]: https://github.com/powerman/slogx/commit/38ff7e5ac74f83e834a756711c87756c8ebd866e
[e18486b]: https://github.com/powerman/slogx/commit/e18486ba5f1d73068eb834ed934a43bb1cea77ae
[d9a940a]: https://github.com/powerman/slogx/commit/d9a940a75b25c81cc44b0dbd775f0b9c6f963a44
[547a2cd]: https://github.com/powerman/slogx/commit/547a2cdd43cd11cba4b33492fec619c9bba2bbb0
[add073f]: https://github.com/powerman/slogx/commit/add073faab20244509f233d650e7446a2d1cc52c
[604942f]: https://github.com/powerman/slogx/commit/604942fbb27ec4add49903ac90b80abd4d0b5fd7
[5e77c56]: https://github.com/powerman/slogx/commit/5e77c564ee458c314a8d50130a5c0cf4ffc89bd9
[74b18b5]: https://github.com/powerman/slogx/commit/74b18b505689cff337d7565f37b4a3b6b24b5ae1
[6cd8193]: https://github.com/powerman/slogx/commit/6cd8193febb4d5dc9e1f9ec93ff164500688eaa6

## [0.5.1] - 2026-11-18

### ⚡ Performance

- Speed up GroupOrAttrs.Record for goa without groups by @powerman in [de2e60f]

### 📦️ Dependencies

- **(deps)** Bump powerman/check to v1.14.1 and golang.org/x/text to v0.44.0 by @powerman in [718bc30]

[0.5.1]: https://github.com/powerman/slogx/compare/v0.5.0..v0.5.1
[de2e60f]: https://github.com/powerman/slogx/commit/de2e60fd17dac547c183cdf3be6b98b42c949f39
[718bc30]: https://github.com/powerman/slogx/commit/718bc300ce31fa9d0f6be15e7bd42401b7a20179

## [0.5.0] - 2026-11-18

### 🚀 Added

- Add WrapHandlerConfig.Resolve by @powerman in [4e1487e]
- Add BootstrapHandler to hold records until handoff by @powerman in [b557904]

### 🔔 Changed

- Export GroupOrAttrs.Handler by @powerman in [8c65307]

### 📚 Documentation

- Add AGENTS.md project instructions and CLAUDE.md wrapper by @powerman in [3243ca1]

[0.5.0]: https://github.com/powerman/slogx/compare/v0.4.0..v0.5.0
[3243ca1]: https://github.com/powerman/slogx/commit/3243ca1cbfdf8d28750711b0f375b56ede99fa51
[4e1487e]: https://github.com/powerman/slogx/commit/4e1487e0f311d36595cb50409aec42614cfe9c5a
[b557904]: https://github.com/powerman/slogx/commit/b55790497b9cc40d284fdd6b60350b8e511dd373
[8c65307]: https://github.com/powerman/slogx/commit/8c6530745807e3e9173977d16cd1b7f7eb92cd5a

## [0.4.0] - 2026-11-14

### 🚀 Added

- Support encoding.TextAppender in LayoutHandler by @powerman in [ebb3a64]
- Add ErrorAttrsOf to log error attrs without its text by @powerman in [c364e5a]

### 🔔 Changed

- Require Go 1.26 by @powerman in [a7814c8]
- Order tasks in mise.toml like the template by @powerman in [19e8804]
- Split joined errors in ErrorAttrs and return message from ErrorAttrsOf by @powerman in [193beda]

### 🐛 Fixed

- Number groups of split errors in output order by @powerman in [4501519]
- Stop appending to caller-owned args slices by @powerman in [1c33c73]

### 📦️ Dependencies

- **(deps)** Upgrade go and golangci-lint by @powerman in [b6fe2fa]
- **(deps)** Upgrade go modules by @powerman in [a04d2ff]
- **(deps)** Upgrade go and golangci-lint by @powerman in [935e404]
- **(deps)** Mise up by @powerman in [17af70f]

[0.4.0]: https://github.com/powerman/slogx/compare/v0.3.0..v0.4.0
[b6fe2fa]: https://github.com/powerman/slogx/commit/b6fe2fab353d92c35920fe0f8aa8eba6809d5354
[a04d2ff]: https://github.com/powerman/slogx/commit/a04d2ffa4dce5adc228f17df85cc0d8f27ae672c
[935e404]: https://github.com/powerman/slogx/commit/935e4044652b8b03c3d37d0542374cfc98469b63
[a7814c8]: https://github.com/powerman/slogx/commit/a7814c8dae2e2deb9fe884fb45d31cdc31e064f4
[ebb3a64]: https://github.com/powerman/slogx/commit/ebb3a64e999fd34dd6e9d4709231804fd3c30854
[17af70f]: https://github.com/powerman/slogx/commit/17af70f2074aac8739129fbe816ed4776a532d47
[19e8804]: https://github.com/powerman/slogx/commit/19e88041e844975dad744b6abd050554a8bb3ac6
[c364e5a]: https://github.com/powerman/slogx/commit/c364e5a2db4bbd6257db47eb79949d5ba61a8d70
[193beda]: https://github.com/powerman/slogx/commit/193bedae49e4c944b090e85b7c3205b9ef912b8d
[4501519]: https://github.com/powerman/slogx/commit/4501519ddc9424b36b824847f0cd27e028738b9f
[1c33c73]: https://github.com/powerman/slogx/commit/1c33c73a05a8c487b01412ad20ce8ba260470d79

## [0.3.0] - 2026-07-12

### 📚 Documentation

- Mention zero external dependencies in README features by @powerman in [665fdc6]

[0.3.0]: https://github.com/powerman/slogx/compare/v0.2.2..v0.3.0
[665fdc6]: https://github.com/powerman/slogx/commit/665fdc6c667b2d924fd6de3038d4b90d82093e39

## [0.2.2] - 2026-04-07

### 📦️ Dependencies

- **(deps)** Upgrade by @powerman in [2619dd2]

[0.2.2]: https://github.com/powerman/slogx/compare/v0.2.1..v0.2.2
[2619dd2]: https://github.com/powerman/slogx/commit/2619dd2bb841b2a3b3197b0acb4e359ab3712d4e

## [0.2.1] - 2025-10-14

### 📚 Documentation

- Improve by @powerman in [710ebd3]

[0.2.1]: https://github.com/powerman/slogx/compare/v0.2.0..v0.2.1
[710ebd3]: https://github.com/powerman/slogx/commit/710ebd345c2bbe1600a0e9d8e1a0ad4c8fc8b659

## [0.2.0] - 2025-10-13

### ➖ Removed

- Remove ParseLevel by @powerman in [6b7b457]
- Remove NewContextWithLogger by @powerman in [9fc3627]

### 🚀 Added

- **(ErrorAttrs)** Add support for errors.Join by @powerman in [#65]
- Add errorAttrs type by @salamandra19 in [#42]
- Add ErrorAttrs options by @salamandra19 in [#45]
- Add LayoutHandler by @powerman in [#66]
- Add NewContextHandler by @powerman in [8e50ccc]
- **⚠️ BREAKING!** Add ContextWith, change ContextWithAttrs signature by @powerman in [3a377c5]
- Add WrapHandler and GroupOrAttrs by @powerman in [#67]
- Add Middleware support by @powerman in [#68]
- Add NewDefaultContextLogger by @powerman in [#69]
- Add Stack, StackSkip by @powerman in [#70]

### 🔔 Changed

- **⚠️ BREAKING!** Rename CtxHandler to ContextHandler by @powerman in [3ab34e8]
- **⚠️ BREAKING!** Unexport NewContextWithHandler and HandlerFromContext by @powerman in [bfc1165]

### 📚 Documentation

- **(README)** Update linter recommendation by @powerman in [0a10661]

[0.2.0]: https://github.com/powerman/slogx/compare/%40%7B10year%7D..v0.2.0
[#42]: https://github.com/powerman/slogx/pull/42
[#45]: https://github.com/powerman/slogx/pull/45
[6b7b457]: https://github.com/powerman/slogx/commit/6b7b457ff3ab72de9fa1a121519a36ce54f30eec
[9fc3627]: https://github.com/powerman/slogx/commit/9fc3627255d8a5ddb6421bd35331c6db44238e03
[#65]: https://github.com/powerman/slogx/pull/65
[#66]: https://github.com/powerman/slogx/pull/66
[3ab34e8]: https://github.com/powerman/slogx/commit/3ab34e88eb59b12017abca49858c1b911f005d2e
[8e50ccc]: https://github.com/powerman/slogx/commit/8e50ccc97811f11e501f9d5127993df46d1cb1ab
[3a377c5]: https://github.com/powerman/slogx/commit/3a377c557565cdd23ce4880ad85ab790e41169fb
[bfc1165]: https://github.com/powerman/slogx/commit/bfc1165d4917f6f155eaced9ba9cca45a4874971
[#67]: https://github.com/powerman/slogx/pull/67
[#68]: https://github.com/powerman/slogx/pull/68
[0a10661]: https://github.com/powerman/slogx/commit/0a10661e3c0a53c6d0b5ad10b746f067c5cfcace
[#69]: https://github.com/powerman/slogx/pull/69
[#70]: https://github.com/powerman/slogx/pull/70

<!-- generated by git-cliff -->
