#!/usr/bin/env node

const fs = require('fs')
const path = require('path')

const args = new Map()
for (let i = 2; i < process.argv.length; i += 1) {
  const arg = process.argv[i]
  if (!arg.startsWith('--')) continue
  const key = arg.slice(2)
  const next = process.argv[i + 1]
  if (next && !next.startsWith('--')) {
    args.set(key, next)
    i += 1
  } else {
    args.set(key, 'true')
  }
}

const platform = args.get('platform')
const version = args.get('version')
const dir = path.resolve(args.get('dir') || 'dist-electron')

function fail(message) {
  console.error(`[verify-release-assets] ${message}`)
  process.exit(1)
}

function listFiles(root) {
  if (!fs.existsSync(root)) {
    fail(`directory not found: ${root}`)
  }

  const files = []
  const walk = (current) => {
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const fullPath = path.join(current, entry.name)
      if (entry.isDirectory()) {
        walk(fullPath)
      } else {
        files.push(fullPath)
      }
    }
  }
  walk(root)
  return files
}

function basenameList(files) {
  return files.map((file) => path.basename(file))
}

function requireFile(files, name) {
  const found = files.find((file) => path.basename(file) === name)
  if (!found) {
    fail(`missing ${name}`)
  }
  return found
}

function requireMatch(names, pattern, label) {
  const found = names.find((name) => pattern.test(name))
  if (!found) {
    fail(`missing ${label}`)
  }
  return found
}

function verifyYml(filePath, expectedVersion, requiredArtifact) {
  const content = fs.readFileSync(filePath, 'utf8')
  if (expectedVersion && !new RegExp(`^version:\\s*${expectedVersion.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*$`, 'm').test(content)) {
    fail(`${path.basename(filePath)} does not contain expected version ${expectedVersion}`)
  }
  if (requiredArtifact && !content.includes(requiredArtifact)) {
    fail(`${path.basename(filePath)} does not reference ${requiredArtifact}`)
  }
}

if (!platform || !['windows', 'linux', 'release'].includes(platform)) {
  fail('usage: verify-release-assets.js --platform windows|linux|release --version <version> [--dir <path>]')
}

const files = listFiles(dir)
const names = basenameList(files)

if (platform === 'windows' || platform === 'release') {
  const exe = requireMatch(names, /^knot-win-.+-x64\.exe$/, 'Windows installer')
  requireMatch(names, /^knot-win-.+-x64\.exe\.blockmap$/, 'Windows blockmap')
  const latestYml = requireFile(files, 'latest.yml')
  verifyYml(latestYml, version, exe)
}

if (platform === 'linux' || platform === 'release') {
  const appImage = requireMatch(names, /^knot-linux-.+-x86_64\.AppImage$/, 'Linux AppImage')
  requireMatch(names, /^knot-linux-.+-x86_64\.AppImage\.blockmap$/, 'Linux AppImage blockmap')
  requireMatch(names, /^knot-linux-.+-amd64\.deb$/, 'Linux deb package')
  const latestLinuxYml = requireFile(files, 'latest-linux.yml')
  verifyYml(latestLinuxYml, version, appImage)
}

console.log(`[verify-release-assets] ${platform} assets ok in ${dir}`)
