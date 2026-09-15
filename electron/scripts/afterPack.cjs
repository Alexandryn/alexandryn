// electron-builder afterPack hook: sets Electron's packaged-binary security
// fuses and reads them back from the actual packaged binary to confirm
// the flip took effect, rather than trusting the config that requested it
// (phase 99's own release standard — see .claude/roadmap/99-release).
'use strict'

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

function electronBinaryPath(appOutDir, platform, executableName) {
  if (platform === 'darwin') {
    return path.join(appOutDir, `${executableName}.app`, 'Contents', 'MacOS', executableName)
  }
  if (platform === 'win32') {
    return path.join(appOutDir, `${executableName}.exe`)
  }
  return path.join(appOutDir, executableName)
}

exports.default = async function afterPack(context) {
  const { appOutDir, packager, electronPlatformName } = context
  const executableName = packager.executableName
  const binaryPath = electronBinaryPath(appOutDir, electronPlatformName, executableName)
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
