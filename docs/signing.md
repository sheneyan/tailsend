# Code signing (Apple notarization and Windows Authenticode)

Unsigned `Tailsend.app` / `.dmg` / `.exe` work for you and for testers who
right-click → Open (macOS) or click through SmartScreen (Windows). **Public
downloads need a paid certificate.** The pack scripts stay unsigned unless
you set the environment variables below.

## What you must buy

| Platform | What | Typical cost | Where |
|---|---|---|---|
| macOS notarization | [Apple Developer Program](https://developer.apple.com/programs/) + **Developer ID Application** certificate | USD 99 / year | Apple Developer → Certificates |
| Windows Authenticode | Code-signing cert from a public CA (DigiCert, Sectigo, SSL.com, …) | OV: roughly USD 100–400 / year. EV: more, usually a USB token | The CA; not Microsoft |

Apple’s free “Apple Development” cert is **not** valid for notarized distribution.
Don’t use an App Store “Mac Distribution” cert for a GitHub dmg either.

Linux GUI tarballs are not signed this way (optional: a detached GPG signature).

## macOS: sign + notarize

One-time:

1. Enroll in the Apple Developer Program (the same Apple ID as Xcode).
2. Developer.apple.com → Account → Certificates → **Developer ID Application**.
   Download and double-click so it lands in Keychain.
3. Confirm:

   ```bash
   security find-identity -v -p codesigning | grep "Developer ID Application"
   ```

   You want a line like `Developer ID Application: Your Name (TEAMID)`.
4. Create an [app-specific password](https://appleid.apple.com) and store it
   for `notarytool`:

   ```bash
   xcrun notarytool store-credentials "tailsend-notary" \
     --apple-id "you@example.com" \
     --team-id TEAMID \
     --password "xxxx-xxxx-xxxx-xxxx"
   ```

Each build (from `desktop/`):

```bash
export CODESIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
export NOTARY_PROFILE="tailsend-notary"
make dmg
```

`pack-macos.sh` then:

1. `codesign --force --deep --options runtime --entitlements Tailsend.entitlements`
2. Builds the dmg
3. Signs the dmg
4. `xcrun notarytool submit … --wait` and `xcrun stapler staple` on the dmg
   (and the `.app`)

Check:

```bash
spctl -a -vv desktop/Tailsend.app
# source=Notarized Developer ID
```

Without `CODESIGN_IDENTITY`, `make dmg` still produces an unsigned dmg (right-click
→ Open the first time).

Hardened runtime needs the entitlements in `desktop/Tailsend.entitlements`
(WebKit/Wails requires JIT). If notarization returns **Invalid**, fetch the log:

```bash
xcrun notarytool log <submission-id> --keychain-profile tailsend-notary
```

## Windows: Authenticode

One-time:

1. Buy an **OV** or **EV** code-signing certificate. EV uses a hardware token
   and gets SmartScreen reputation faster; OV is cheaper but SmartScreen may
   warn “unknown publisher” until the file is downloaded often.
2. Install the cert (or plug in the token). Install [Windows SDK](https://developer.microsoft.com/windows/downloads/windows-sdk/)
   so `signtool.exe` is on `PATH`.

Each build:

```powershell
$env:SIGN_THUMBPRINT = "YOURCERTSHA1THUMBPRINT"
cd D:\tailsend\desktop
powershell -ExecutionPolicy Bypass -File .\pack-windows.ps1
```

Thumbprint: `certmgr.msc` → Personal → Certificates → details → Thumbprint
(no spaces). Optional timestamp URL:

```powershell
$env:SIGN_TIMESTAMP = "http://timestamp.digicert.com"
```

`pack-windows.ps1` runs `signtool sign /fd SHA256 /td SHA256 /tr … /sha1 …`
when `SIGN_THUMBPRINT` is set. If `signtool` is missing, the script still
builds an unsigned exe.

Verify:

```powershell
signtool verify /pa .\Tailsend.exe
```

Do **not** `go install github.com/tc-hib/go-winres` on a Defender-heavy machine.

## GitHub Release

CLI: `v*` tag → `tailsend-cli-*` (unsigned Go binaries; usually fine).

GUI: attach `desktop/dist-release/Tailsend.dmg` and `Tailsend.exe` built on
that OS after signing. GitHub Actions cannot notarize without your Apple
API key in secrets, and cannot Authenticode-sign without the cert/token on
the runner.
