// electron-builder afterPack hook: sets Electron's packaged-binary security
// fuses and reads them back from the actual packaged binary to confirm
// the flip took effect, rather than trusting the config that requested it
// (phase 99's own release standard — see .claude/roadmap/99-release).
'use strict'

const fs = require('node:fs')
const path = require('node:path')
const {
  flipFuses,
  getCurrentFuseWire,
  FuseVersion,
  FuseV1Options,
  FuseState,
} = require('@electron/fuses')

// Recommended posture (CLAUDE.md, phase 99): RunAsNode off,
// EnableNodeCliInspectArguments off, EnableNodeOptionsEnvironmentVariable
// off, EnableCookieEncryption on, OnlyLoadAppFromAsar on,
// EnableEmbeddedAsarIntegrityValidation on (macOS/Windows only — the fuse
// ties into code-signing verification, which Linux packages don't have).
function desiredFuses(platform) {
  const fuses = {
    [FuseV1Options.RunAsNode]: false,
    [FuseV1Options.EnableCookieEncryption]: true,
    [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
    [FuseV1Options.EnableNodeCliInspectArguments]: false,
    [FuseV1Options.OnlyLoadAppFromAsar]: true,
  }
  if (platform === 'darwin' || platform === 'win32') {
    fuses[FuseV1Options.EnableEmbeddedAsarIntegrityValidation] = true
  }
  return fuses
}

// electron-builder only fills `packager.executableName` on Linux. On Windows and
// macOS it is undefined, and the binary is named after productName instead
// (the first release run built "undefined.exe" from it). So the names to try are
// every candidate the packager offers, and the binary is the first path that
// really exists: a wrong guess fails loudly instead of flipping fuses on nothing.
function resolveBinaryPath({ appOutDir, platform, names }) {
  const candidates = [...new Set(names.filter(Boolean))]
  if (candidates.length === 0) {
    throw new Error(`afterPack: no executable name to look for on ${platform}`)
  }
  const paths = []
  for (const name of candidates) {
    if (platform === 'darwin') {
      for (const bundle of candidates) {
        paths.push(path.join(appOutDir, `${bundle}.app`, 'Contents', 'MacOS', name))
      }
    } else if (platform === 'win32') {
      paths.push(path.join(appOutDir, `${name}.exe`))
    } else {
      paths.push(path.join(appOutDir, name))
    }
  }
  const found = paths.find((candidate) => fs.existsSync(candidate))
  if (!found) {
    throw new Error(
      `afterPack: no Electron binary found for ${platform}; tried:\n${paths.join('\n')}`,
    )
  }
  return found
}

exports.default = async function afterPack(context) {
  const { appOutDir, packager, electronPlatformName } = context
  const binaryPath = resolveBinaryPath({
    appOutDir,
    platform: electronPlatformName,
    names: [
      packager.executableName,
      packager.config?.executableName,
      packager.appInfo?.productFilename,
    ],
  })
  const fuses = desiredFuses(electronPlatformName)

  await flipFuses(binaryPath, {
    version: FuseVersion.V1,
    resetAdHocDarwinSignature: electronPlatformName === 'darwin',
    ...fuses,
  })

  // Read the fuses back from the packaged binary itself — proves the flip
  // took effect on this exact artifact, not just that flipFuses returned
  // without throwing. getCurrentFuseWire reports each fuse as a
  // FuseState (the raw ASCII byte '0'/'1' embedded in the binary's fuse
  // wire, i.e. 48/49), never a plain boolean — comparing it directly
  // against `fuses`' booleans would silently never match.
  const actual = await getCurrentFuseWire(binaryPath)
  const mismatches = []
  for (const [option, expected] of Object.entries(fuses)) {
    const expectedState = expected ? FuseState.ENABLE : FuseState.DISABLE
    if (actual[option] !== expectedState) {
      mismatches.push(
        `fuse ${option}: expected ${expected} (${expectedState}), packaged binary has ${actual[option]}`,
      )
    }
  }
  if (mismatches.length > 0) {
    throw new Error(
      `afterPack: fuse verification failed for ${electronPlatformName} at ${binaryPath}:\n` +
        mismatches.join('\n'),
    )
  }

  console.log(
    `afterPack: fuses verified on packaged ${electronPlatformName} binary (${binaryPath})`,
  )
}

exports.resolveBinaryPath = resolveBinaryPath
