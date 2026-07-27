# Osaurus architecture reference (for building a new native macOS SwiftUI app)

Source: `/Users/dennison/develop/fork-osaurus` (read-only survey). All paths below are relative to that repo root unless stated otherwise.

---

## 1. Project layout

**Both Xcode-project-based and SPM-based, wired together by a workspace.** There is no standalone `Package.swift` at the repo root — the root is `osaurus.xcworkspace`, which references:

```xml
<!-- osaurus.xcworkspace/contents.xcworkspacedata -->
<FileRef location="group:App/osaurus.xcodeproj"/>
<FileRef location="group:Packages/OsaurusCore"/>
<FileRef location="group:Packages/OsaurusCLI"/>
<FileRef location="group:Packages/OsaurusRepository"/>
```

- `App/osaurus.xcodeproj` — the actual macOS app target. Very thin: `App/osaurus/osaurusApp.swift` (entry point + menu commands), `AppIntents/`, `Assets.xcassets`, `Info.plist`, `osaurus.entitlements`, an app icon (`.icon` bundle, new Xcode 16+ format). **Almost no business logic lives here.**
- `Packages/OsaurusCore` — an SPM library that contains essentially the entire app: `AppDelegate.swift`, all `Views/`, `Managers/`, `Services/`, `Networking/` (the embedded HTTP/SSE server), `Storage/`, `Tools/`, `Subagent/`, `Identity/`, `PrivacyFilter/`, `ComputerUse/`, `AppleScript/`, plus a vendored `SQLCipher/` C target and `ObjCSupport/` (Obj-C shim for catching NSExceptions Swift can't catch). The Xcode app target just links this package and calls into it.
- `Packages/OsaurusCLI` — a separate SPM executable (`osaurus-cli`) plus a `OsaurusCLICore` library, for the command-line companion binary that gets embedded into the `.app` bundle's `Contents/Helpers/`.
- `Packages/OsaurusRepository` — shared low-level types pulled in by both `OsaurusCore` and `OsaurusCLI`.
- `Packages/OsaurusNetworking` — a separate small SPM library, path-dependency of `OsaurusCore`.
- `Packages/OsaurusEvals`, `Packages/OsaurusPlugins`, `Packages/OsaurusPluginTestKit` — auxiliary packages (eval harness, plugin system, plugin test scaffolding). Not part of the xcworkspace's file refs (built directly via `swift run --package-path Packages/OsaurusEvals ...` instead), which keeps Xcode's own indexing/build graph smaller.

**Pattern to steal:** put nearly all Swift source in one big local SPM package (`OsaurusCore`-style) that the Xcode project just depends on. This gets you `swift build`/`swift test` speed and SwiftPM dependency management, while still producing a proper signed/notarized `.app` via Xcode/xcodebuild. Split out a second package only for genuinely separate binaries (their CLI) or a truly independent library with no AppKit/SwiftUI dependency (`OsaurusNetworking`).

**`Package.swift` shape** (`Packages/OsaurusCore/Package.swift`):
```swift
// swift-tools-version: 6.2
let package = Package(
    name: "OsaurusCore",
    defaultLocalization: "en",
    platforms: [.macOS(.v15)],
    products: [.library(name: "OsaurusCore", targets: ["OsaurusCore"])],
    dependencies: [ /* ~20 remote SPM deps: swift-nio, MCP swift-sdk, Sparkle, vmlx-swift, FluidAudio, VecturaKit, swift-secp256k1, SwiftMath, Highlightr, AAChartKit-Swift, aptabase-swift, sentry-cocoa, containerization, IkigaJSON, eventsource, plus local path deps: ../OsaurusNetworking, ../OsaurusRepository */ ],
    targets: [
        .target(name: "OsaurusSQLCipher", path: "SQLCipher", sources: ["sqlite3.c"], publicHeadersPath: "include", cSettings: [...]),  // vendored C lib
        .target(name: "OsaurusObjCSupport", path: "ObjCSupport", publicHeadersPath: "include"),  // Obj-C shim, kept separate because a SwiftPM target can't mix Swift+ObjC
        .target(name: "OsaurusCore", dependencies: [...], path: ".", exclude: ["Tests", "SQLCipher", "ObjCSupport"], resources: [.process("Resources")]),
        .testTarget(name: "OsaurusCoreTests", dependencies: [...], path: "Tests", resources: [.process("ComputerUse/Fixtures")]),
    ]
)
```
Key idioms worth copying: `path: "."` on the main target with `exclude:` for sibling-target directories (lets one folder host multiple SwiftPM targets); a separate ObjC target only for NSException-catching shims (SwiftPM targets can't mix languages); vendoring a C library (SQLCipher) as its own target with careful header-guard `cSettings` when it collides with a system framework (their long comment about `Fts5ExtensionApi` typedef collisions is a good example of the kind of gotcha to expect when vendoring C libs alongside a Swift dependency graph that also touches the same C library, here `vmlx-swift`'s use of system SQLite3).

---

## 2. Build & run tooling (no Xcode GUI needed)

All from `Makefile` (repo root) and `scripts/`:

```bash
# Build the CLI helper binary
make cli                       # xcodebuild -workspace osaurus.xcworkspace -scheme osaurus-cli -configuration Release -derivedDataPath build/DerivedData build -quiet

# Build the app (depends on cli) + embed the CLI into Contents/Helpers
make app

# Install a dev symlink for the CLI at /usr/local/bin/osaurus
make install-cli                # runs ./scripts/release/install_cli_symlink.sh --dev

# Run the server from the CLI (builds+installs first)
make serve PORT=1337 EXPOSE=1   # -> osaurus serve --port 1337 --expose
make status                     # -> osaurus status

# Fast unit test loop (SwiftPM only, no xcodebuild)
make test                       # swift test --package-path Packages/OsaurusCore

# Full CI-equivalent test run (mirrors CI's test-core job exactly)
make ci-test                    # xcodebuild test -workspace osaurus.xcworkspace -scheme OsaurusCoreTests -resultBundlePath build/Tests.xcresult -quiet -skipPackagePluginValidation -skipMacroValidation -enableCodeCoverage NO ... | xcbeautify --renderer terminal
# then: open build/Tests.xcresult   # gives Xcode's Test Navigator UI without opening the project

make clean                      # rm -rf build/DerivedData
```

Lint/format (not Make targets — run directly, and gated by `lefthook`):
```bash
swift-format lint --strict --recursive Packages App
swift-format format --in-place --recursive Packages App     # autofix
swiftlint lint --reporter github-actions-logging             # config: .swiftlint.yml (scoped to Packages/OsaurusCore, OsaurusCLI, OsaurusRepository, App)
```
`.swift-format` config: 4-space indent, 120 col line length, 1 blank line max, `lineBreakBeforeEachArgument: true`. `.swiftlint.yml` disables the usual noisy rules (`line_length`, `type_body_length`, `function_body_length`, `cyclomatic_complexity`, `identifier_name`, `type_name`, `trailing_comma`, ...) and opts into a short list of correctness/style rules (`empty_count`, `closure_spacing`, `first_where`, `redundant_nil_coalescing`, ...).

`lefthook.yml` runs `swift-format lint --strict` as a **pre-push** git hook (not pre-commit) — cheap to steal wholesale.

Packaging / signing / notarizing (`scripts/build/`, `scripts/release/`, driven by `.github/workflows/build-and-release.yml`, not usually run manually):
- `scripts/build/build_arm64.sh` — arm64 release build
- `scripts/build/install_certificates.sh` — imports signing cert into a CI keychain
- `scripts/build/verify_signing.sh` — checks codesign validity
- `scripts/build/verify_launch.sh` — actually launches the built app with `OSAURUS_SPAWN_CHECK=1` and expects a sentinel stdout line + exit 0, to catch AMFI/entitlement rejections before shipping (see `osaurusApp.swift`'s `OSAURUS_SPAWN_CHECK` handling below)
- `scripts/build/notarize.sh` — `xcrun notarytool` submit + staple
- `scripts/build/create_dmgs.sh` — builds the `.dmg` (uses `assets/dmg-bg.tiff` as background art)
- `scripts/build/package_dsyms.sh` / `upload_dsyms_sentry.sh` — dSYM packaging + Sentry symbol upload
- `scripts/release/install_cli_symlink.sh` — installs/updates the `/usr/local/bin/osaurus` symlink (dev and prod modes)
- `scripts/release/generate_and_deploy_appcast.sh` + `.github/workflows/deploy-appcast.yml` — Sparkle appcast generation/publish
- `scripts/release/generate_acknowledgements.py` — generates `Acknowledgements.json`/`Credits.rtf` from `Package.resolved`

**Steal-worthy pattern:** `OSAURUS_SPAWN_CHECK=1` as a fast, no-UI smoke test hook baked directly into `@main`:
```swift
// App/osaurus/osaurusApp.swift
static func main() {
    if ProcessInfo.processInfo.environment["OSAURUS_SPAWN_CHECK"] == "1" {
        print("OSAURUS_SPAWN_OK")
        exit(0)
    }
    signal(SIGPIPE, SIG_IGN)   // ignore SIGPIPE so socket/pipe writes fail as EPIPE, not process death
    osaurusApp.main()
}
```

---

## 3. Testing

- **Framework:** XCTest (not swift-testing). Test file names use the `...Tests.swift` XCTestCase convention throughout `Packages/OsaurusCore/Tests/` and `Packages/OsaurusCLI/Tests/`.
- **Location:** `Packages/OsaurusCore/Tests/` (mirrors the source tree: `Tests/Storage/`, `Tests/Plugin/`, `Tests/ComputerUse/Fixtures/` for test resources, etc.), `Packages/OsaurusCLI/Tests/`, `Packages/OsaurusEvals/` has its own test target.
- **Headless run (exact commands):**
  ```bash
  swift test --package-path Packages/OsaurusCore
  swift test --package-path Packages/OsaurusCLI --parallel
  OSAURUS_DISABLE_KEYCHAIN_FOR_TESTS=1 swift test --package-path Packages/OsaurusEvals
  ```
  For Keychain-touching tests, avoid OS Keychain prompts with:
  ```bash
  OSAURUS_DISABLE_KEYCHAIN_FOR_TESTS=1 \
  OSAURUS_TEST_ROOT=/tmp/osaurus-test \
  OSU_MODELS_DIR=/tmp/osaurus-test-models \
  make test
  ```
  (`OSU_MODELS_DIR` matters if you have real local model files — without the override, model-dispatch tests try to load a real model inside the SwiftPM test harness, which has no Metal kernels, and crash. Some Keychain-gated suites, e.g. `PluginAgentScopingTests`, must be run *without* the disable flag to get real proof.)
- **CI-exact repro:** `make ci-test` runs the identical `xcodebuild test -scheme OsaurusCoreTests` invocation CI uses, piped through `xcbeautify`, writing `build/Tests.xcresult` (openable with `open build/Tests.xcresult` for the graphical Test Navigator without opening the whole project).
- **UI/snapshot testing:** `App/osaurusUITests/` exists (`osaurusUITests.swift`, `osaurusUITestsLaunchTests.swift`) — standard XCUITest launch tests, not visual snapshot testing. No third-party snapshot-testing library (e.g. swift-snapshot-testing) is used.
- **CI workflows** (`.github/workflows/`):
  - `ci.yml` (650 lines) — the big one. Jobs: `test-core` (pinned `macos-26` runner, Xcode `26.4.1`, `xcodebuild test -scheme OsaurusCoreTests`, aggressive SPM+DerivedData caching keyed by a `CACHE_SALT` env var that can be bumped to force a cold rebuild), `test-cli` (`swift test --package-path Packages/OsaurusCLI --parallel`), an evals-package test job (`swift test --package-path Packages/OsaurusEvals`), `swiftlint` (`brew install swiftlint && swiftlint lint --reporter github-actions-logging`), and a `shellcheck` job (referenced as a required check).
  - `build-and-release.yml` — triggered on semver tags; gates the ~37-minute macOS release build behind a cheap `ubuntu-latest` job that queries GitHub's check-runs API to require `test-core`, `test-cli`, `swiftlint`, `shellcheck` all green on the tagged commit before spending release-runner minutes. Then signs, notarizes, DMGs, uploads dSYMs to Sentry, generates release notes.
  - `deploy-appcast.yml` — regenerates/publishes the Sparkle `appcast.xml`.
  - `label-merged.yml`, `release-drafter.yml`, `sandbox-image.yml` — housekeeping/automation, not build-critical.

---

## 4. SwiftUI patterns

**Menu bar app, not a document/window app.** `App/osaurus/osaurusApp.swift`:
```swift
struct osaurusApp: SwiftUI.App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    var body: some SwiftUI.Scene {
        Settings { EmptyView() }   // no real Settings scene; window management is manual via AppDelegate
        .commands { fileMenuCommands; fileMenuExtras; settingsCommand; aboutCommand; viewMenuCommands; windowMenuCommands; helpMenuCommands }
    }
}
```
- The `Settings` scene is a deliberate no-op placeholder (`EmptyView()`); all real windows (chat windows, the management/settings window, etc.) are opened imperatively through `AppDelegate` / `WindowManager` / `ChatWindowManager` (`NSWindowController`-style, not SwiftUI `WindowGroup`).
- Almost all app behavior is expressed as SwiftUI `.commands { CommandGroup(...) }` blocks replacing/extending the default menu (`CommandGroup(replacing: .newItem)`, `.appSettings`, `.appInfo`, `.help`, and `CommandGroup(after: .sidebar/.newItem/.windowList)`), wired to `@MainActor` `Task` blocks that call into singleton managers.
- `AppDelegate` (`Packages/OsaurusCore/AppDelegate.swift`, `NSObject, NSApplicationDelegate, NSPopoverDelegate`) owns the actual UI chrome: builds an `NSStatusItem` (`NSStatusBar.system.statusItem`) with a custom icon and two small overlay "dot" indicator views (server-busy = green, VAD-listening = blue), and a click handler that toggles an `NSPopover`. This is the classic menu-bar-extra pattern done with raw AppKit rather than SwiftUI's `MenuBarExtra` scene — gives more control over the popover/positioning than `MenuBarExtra` currently allows.
- **State management:** `ObservableObject` (45 conformers under `Managers`/`Services`) is the dominant pattern, not the newer `@Observable` macro (only 5 conformers) and not TCA. Almost everything is a `@MainActor` singleton `XManager.shared` / `XService.shared` (37 files under `Managers/` are `@MainActor`), injected into views via `@ObservedObject` at the app-struct level (`ThemeManager.shared`, `VADService.shared`) or `.environmentObject(updater)` per-window. No Combine-heavy reactive chains found; managers just publish `@Published` state.
- **Navigation:** no NavigationStack/coordinator abstraction; it's a "many independent windows + one settings/management window with an internal tab enum" structure — `ManagementTab` (`.themes`, `.models`, `.plugins`, `.server`, `.schedules`, `.watchers`, `.agents`, ...) selects a pane inside one `showManagementWindow(initialTab:)` window, while chat conversations are separate `ChatWindowManager.shared.createWindow(agentId:)` windows (multi-window, not single-window-with-navigation).
- View tree lives under `Packages/OsaurusCore/Views/{Agent,Chat,Common,Credits,Identity,ImageGeneration,Insights,Management,Memory,Model,Onboarding,Pairing,Plugin,Provider,Router,Sandbox,Schedule,Settings,Skill,SlashCommand,Theme,Toast,Voice,Watcher,WhatsNew}/` — one directory per feature domain, flat SwiftUI view files inside.

---

## 5. Networking

- **Not URLSession-based for the server side** — Osaurus embeds its own HTTP/SSE server using **SwiftNIO** directly (`swift-nio` dependency: `NIOCore`, `NIOHTTP1`, `NIOPosix`). Server code lives in `Packages/OsaurusCore/Networking/`: `OsaurusServer.swift` (top-level server object), `HTTPHandler.swift` (the big NIO `ChannelInboundHandler`, request routing, streaming response writers), `HTTPRequestParse.swift`, `HTTPProtocolErrors.swift`, `HTTPLoopHelpers.swift`, `RequestValidator.swift`, `ServerController.swift` (start/stop/status), `SocketAddress+Loopback.swift`, `SecureChannelResponseEncryptor.swift`, `RelayTunnelManager.swift`, plus Bonjour/mDNS discovery (`BonjourAdvertiser.swift`, `BonjourBrowser.swift`, using `NSBonjourServices` in `Info.plist`) and `GlobalProxyConfiguration.swift`/`GlobalProxySettings.swift` for outbound proxy config.
- **SSE streaming:** hand-rolled, via a `SSEResponseWriter` type used throughout `HTTPHandler.swift`. Pattern: check `req.stream == true || Accept header contains "text/event-stream"` (`wantsSSE`), write a 200 head immediately, then stream `data: ...\n\n` chunks; a periodic `startSSEKeepalive` task emits `: ping\n\n` comment lines every 15s to keep idle connections alive; if a failure happens mid-stream, since the 200 head is already committed, errors are surfaced as an **in-band SSE error chunk** rather than an HTTP error status (documented repeatedly in comments — a real gotcha to plan for if you build your own SSE server: you cannot flip to a 5xx after headers are sent).
- **Outbound HTTP** (calling remote LLM providers etc.) is layered on top of the same NIO/EventSource stack: the `eventsource` package (`mattt/eventsource`) is pulled in with its `AsyncHTTPClient` trait enabled specifically so remote SSE consumption (from provider APIs) shares the NIO graph rather than adding a second HTTP stack. `async-http-client` and `swift-nio-ssl`/`swift-nio-http2`/`swift-nio-extras` show up in `Package.resolved` as transitive deps of that.
- **gRPC:** `grpc-swift-2`, `grpc-swift-nio-transport`, `grpc-swift-protobuf` are present in `Package.resolved` (transitive, likely via `containerization` for sandboxed tool execution — not a networking layer the app authors call directly).

**Full remote SPM dependency list** (from `Packages/OsaurusCore/Package.resolved`, identity → URL):
```
aachartkit-swift        https://github.com/AAChartModel/AAChartKit-Swift.git       (charts, for Insights views)
aptabase-swift          https://github.com/aptabase/aptabase-swift.git             (product analytics, consent-gated)
async-http-client       https://github.com/swift-server/async-http-client.git      (transitive)
containerization        https://github.com/apple/containerization.git              (Apple's container/VM framework — sandboxed tool exec)
eventsource             https://github.com/mattt/eventsource.git                    (SSE client, AsyncHTTPClient trait enabled)
fluidaudio              https://github.com/FluidInference/FluidAudio.git            (on-device speech: transcription + TTS)
grpc-swift-2 / grpc-swift-nio-transport / grpc-swift-protobuf  (transitive, via containerization)
highlightr              https://github.com/raspu/Highlightr                        (syntax highlighting for code blocks)
ikigajson               https://github.com/orlandos-nl/IkigaJSON                    (fast JSON)
sentry-cocoa            https://github.com/getsentry/sentry-cocoa.git              (crash/hang reporting, consent-gated)
sparkle                 https://github.com/sparkle-project/Sparkle                  (auto-update)
swift-algorithms / swift-argument-parser / swift-asn1 / swift-async-algorithms /
swift-atomics / swift-certificates / swift-collections / swift-configuration /
swift-crypto / swift-distributed-tracing / swift-http-structured-headers /
swift-http-types / swift-log / swift-nio(+extras/http2/ssl/transport-services) /
swift-numerics / swift-protobuf / swift-service-context / swift-service-lifecycle /
swift-syntax / swift-system                                                          (mostly transitive, Apple SSWG ecosystem)
swift-sdk               https://github.com/modelcontextprotocol/swift-sdk.git       (official MCP Swift SDK)
swift-secp256k1         https://github.com/21-DOT-DEV/swift-secp256k1              (crypto/identity — see docs/IDENTITY.md)
swiftmath               https://github.com/mgriebling/SwiftMath                     (LaTeX/math rendering in chat)
vecturakit              https://github.com/rryam/VecturaKit                        (vector store for memory/RAG)
vmlx-swift              https://github.com/osaurus-ai/vmlx-swift                    (pinned by *commit revision*, not tag — their own MLX inference fork; consolidates MLX/MLXLLM/MLXVLM/tokenizers/etc.)
yyjson / zstd                                                                        (transitive)
```
Not relevant to a generic app unless you're also doing local LLM inference (`vmlx-swift`, `FluidAudio`, `VecturaKit`) or MCP (`swift-sdk`). The broadly reusable picks: **Sparkle** (auto-update), **sentry-cocoa** (crash reporting), **aptabase-swift** (lightweight consent-gated analytics), **Highlightr** (code syntax highlighting), **swift-nio** only if you actually need an embedded server.

---

## 6. macOS integration worth stealing

- **Sandboxing/entitlements:** `App/osaurus/osaurus.entitlements` has **no `com.apple.security.app-sandbox` key at all** — the app is unsandboxed (direct/notarized distribution outside the App Store, consistent with it running a local server, spawning subprocesses/AppleScript, and needing broad filesystem access). Entitlements present: `com.apple.security.automation.apple-events`, `com.apple.security.cs.disable-library-validation`, `com.apple.security.network.client`, `com.apple.security.network.server`, `com.apple.security.personal-information.{addressbook,calendars,location}`, `com.apple.security.device.audio-input`, `com.apple.security.virtualization`. If you need App Store distribution, you'd need the sandbox entitlement plus per-resource temporary exceptions instead — this file is a template for the *unsandboxed* case only.
- **Info.plist usage-description keys**: exhaustive per-permission strings (`NSAppleEventsUsageDescription`, `NSCalendarsUsageDescription`/`NSCalendarsFullAccessUsageDescription`, `NSContactsUsageDescription`, `NSRemindersUsageDescription`, `NSLocationWhenInUseUsageDescription`/`NSLocationUsageDescription`, `NSMicrophoneUsageDescription`, `NSScreenCaptureUsageDescription`, `NSLocalNetworkUsageDescription`) — good checklist to copy verbatim and edit if your app touches any of these.
- **Bonjour/mDNS:** `NSBonjourServices: ["_osaurus._tcp"]` in `Info.plist` + `BonjourAdvertiser.swift`/`BonjourBrowser.swift` for LAN peer discovery between devices running the app.
- **Keychain:** no single "KeychainService" — instead small per-domain wrapper types implementing a protocol, e.g. `KeychainAgentChannelSecretResolver` (`Services/AgentChannel/AgentChannelCustomJSONRunner.swift`) and `KeychainChannelCredentialVaultBackingStore` (`Services/Channels/ChannelCredentialVault.swift`), plus `RemoteProviderKeychain` for provider API keys. The `OSAURUS_DISABLE_KEYCHAIN_FOR_TESTS` env-var pattern (wrappers no-op instead of calling `SecItemAdd`/`SecItemCopyMatching`/etc. when set) is a clean, cheap way to keep tests from popping OS Keychain-access dialogs — worth copying directly.
- **Notifications:** `Services/NotificationService.swift`, a `@MainActor` `UNUserNotificationCenterDelegate` singleton. Notable gotcha they guard against: `UNUserNotificationCenter.current()` throws an Obj-C exception in processes with no app bundle (SwiftPM test runs, their `osaurus-evals` CLI) — they lazily resolve `center` only `if Bundle.main.bundleIdentifier != nil`, and every post-method no-ops when `center` is nil. Copy this guard if any headless binary/test target might touch a class that assumes an app bundle exists.
- **Deep links / URL scheme:** two custom URL schemes registered in `Info.plist` (`CFBundleURLTypes`): `osaurus://` (own deep link scheme) and `huggingface://` (intercepts Hugging Face's own deep link for model install flows).
- **Auto-update:** Sparkle, wired through `SUFeedURL` (`https://osaurus-ai.github.io/osaurus/appcast.xml`) + `SUPublicEDKey` (EdDSA public key for appcast signature verification) in `Info.plist`, `SUEnableAutomaticChecks = true`. App-side: an `UpdaterViewModel` owned by `AppDelegate`, `.checkForUpdatesInBackground()` fired once at launch (lazy-instantiated to avoid arming Sparkle before the app is ready), injected into views via `.environmentObject(updater)`.
- **Settings/management window:** not SwiftUI's `Settings {}` scene (that's an intentional empty placeholder) — a single custom window opened via `AppDelegate.showManagementWindow(initialTab:)`, internally tabbed by a `ManagementTab` enum (`.themes .models .plugins .server .schedules .watchers .agents`). If you want a real multi-pane settings window with full control over presentation (not just gear-icon-in-dock behavior), this pattern — one `NSWindow` + an internal enum-driven tab view — is more flexible than fighting SwiftUI's `Settings` scene.
- **Status item indicator dots:** `AppDelegate.installStatusItem()` overlays two small colored `NSView` "dot" indicators directly on the `NSStatusBarButton` (bottom-trailing = busy/green, top-trailing = VAD-listening/blue), each a `wantsLayer`-backed circular view toggled/animated independently of the base icon — a reusable technique for showing live status without redrawing the whole icon.
- **AppleScript automation:** `Packages/OsaurusCore/AppleScript/` — the app exposes/consumes AppleScript automation (paired with the `NSAppleEventsUsageDescription` entitlement/usage string above); see `docs/AGENT_LOOP.md`/App Intents section below for the sibling App Intents surface.
- **App Intents / Shortcuts:** `App/osaurus/AppIntents/{AgentEntity,OsaurusIntents,OsaurusShortcuts}.swift` — native Shortcuts app integration (see `docs/APP_INTENTS.md`).

---

## 7. AGENTS.md / docs conventions worth adopting

`AGENTS.md` (repo root) is short and points to a global `~/AGENTS.md` for shared conventions, then lists only the project-specific "Build & Test" section — i.e., **don't duplicate global agent conventions per-repo; keep the per-repo file to what's actually different**, mirroring the canonical build commands verbatim from the `Makefile` (exact commands, not paraphrased) plus the Keychain-test env-var recipe reproduced above. The rest of `AGENTS.md` is a long, very domain-specific "Model Runtime Non-Negotiables" section (rules against faking LLM inference correctness) — not transferable to a generic app, but the *pattern* — a dedicated "hard invariants nobody may quietly work around" section — is worth keeping for whatever your new app's actual non-negotiables are (e.g., data integrity, security boundaries).

`docs/` is large (~90 files) and organized as **one Markdown file per feature/investigation/spec**, not a wiki hierarchy — e.g. `docs/SANDBOX.md`, `docs/STORAGE.md`, `docs/MEMORY.md`, `docs/APP_INTENTS.md`, `docs/SECURITY.md`, `docs/PRODUCTION_READINESS.md`, alongside dated investigation/evidence reports (`docs/STEP37_OSAURUS_E2E_EVIDENCE_2026_05_30.md`, `docs/CACHE_WINDOW_INVESTIGATION.md`). Two conventions worth copying directly:
- `docs/CONTRIBUTING.md` + `docs/CODE_OF_CONDUCT.md` + `docs/SUPPORT.md` — standard OSS scaffolding, present and current.
- A `docs/runbooks`-equivalent split between **stable reference docs** (feature/subsystem specs) and **dated evidence reports** (investigation logs with a date in the filename) — keeps living documentation separate from point-in-time proof, avoiding the trap of stale "as of" claims creeping into reference docs.

`.cursor/settings.json` exists too (editor-specific config, not reviewed in depth — low priority to copy unless you also use Cursor).

---

## Bottom line: what to actually copy for a new app

1. **Layout:** one root `<AppName>.xcworkspace` referencing a thin `App/<AppName>.xcodeproj` (entry point, `Info.plist`, entitlements, `.commands` menu wiring only) + one big local SPM package (`<AppName>Core`) holding all real source, `Views/` split by feature domain, `Managers/`+`Services/` as `@MainActor` `ObservableObject` singletons. Split out a second package only for a genuinely separate binary (CLI) or dependency-free shared library.
2. **Tooling:** a `Makefile` with `cli`/`app`/`test`/`ci-test`/`clean` targets wrapping `xcodebuild`/`swift test`/`swift build`, `swift-format` + `swiftlint` with permissive `.swiftlint.yml` (turn off the noisy structural rules), `lefthook.yml` running `swift-format lint --strict` pre-push.
3. **Testing:** XCTest, `Tests/` mirroring source layout, a `make ci-test` target that reproduces the CI xcodebuild invocation exactly (same flags, `xcbeautify`, `.xcresult` bundle) so failures are locally reproducible.
4. **CI:** pin the macOS runner image and Xcode version explicitly (never `macos-latest`), cache SPM+DerivedData with a manually-bumpable cache-salt env var, gate tag-triggered releases on GitHub's check-runs API rather than trusting the tag alone.
5. **SwiftUI:** if it's a menu-bar-first app, use raw `NSStatusItem`+`NSPopover` in an `NSApplicationDelegate` rather than `MenuBarExtra` for more control; keep `Settings {}` as an intentional no-op and manage a real settings window yourself if you need multi-pane tabs; standard `ObservableObject` singletons are simpler to onboard onto than `@Observable`/TCA for this shape of app.
6. **Integration:** Sparkle for auto-update, Sentry for crash reporting, per-domain Keychain wrapper protocols with a `DISABLE_KEYCHAIN_FOR_TESTS`-style escape hatch, exhaustive Info.plist usage-description strings, and — if you skip App Sandbox — don't forget you also skip the safety net; document why in the entitlements file or nearby docs.
