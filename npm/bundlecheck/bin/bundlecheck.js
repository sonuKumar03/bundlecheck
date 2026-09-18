#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const os = require('os');
const https = require('https');
const { execSync, spawnSync } = require('child_process');

const pkg = require('../package.json');
const VERSION = pkg.version;
const REPO = 'sonuKumar03/bundlecheck';

const PLATFORM_MAP = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows'
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64'
};

function getBinaryName() {
  return process.platform === 'win32' ? 'bundlecheck.exe' : 'bundlecheck';
}

function getArchiveName(platform, arch) {
  const osName = PLATFORM_MAP[platform];
  const archName = ARCH_MAP[arch];
  if (!osName || !archName) {
    throw new Error(`Unsupported platform: ${platform} ${arch}`);
  }
  const ext = osName === 'windows' ? 'zip' : 'tar.gz';
  return `bundlecheck_${VERSION}_${osName}_${archName}.${ext}`;
}

function getCacheDir() {
  const home = os.homedir();
  const dir = path.join(home, '.bundlecheck', 'bin', `v${VERSION}`);
  fs.mkdirSync(dir, { recursive: true });
  return dir;
}

function downloadUrl(url, destPath) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destPath);
    const request = (targetUrl) => {
      https.get(targetUrl, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return request(res.headers.location);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`Failed to download binary: HTTP ${res.statusCode} from ${targetUrl}`));
        }
        res.pipe(file);
        file.on('finish', () => {
          file.close(resolve);
        });
      }).on('error', (err) => {
        fs.unlink(destPath, () => {});
        reject(err);
      });
    };
    request(url);
  });
}

function extractArchive(archivePath, destDir) {
  if (archivePath.endsWith('.tar.gz')) {
    execSync(`tar -xzf "${archivePath}" -C "${destDir}"`, { stdio: 'ignore' });
  } else if (archivePath.endsWith('.zip')) {
    if (process.platform === 'win32') {
      execSync(`powershell -command "Expand-Archive -Path '${archivePath}' -DestinationPath '${destDir}' -Force"`, { stdio: 'ignore' });
    } else {
      execSync(`unzip -o "${archivePath}" -d "${destDir}"`, { stdio: 'ignore' });
    }
  }
}

async function ensureBinary() {
  const binaryName = getBinaryName();
  const cacheDir = getCacheDir();
  const binaryPath = path.join(cacheDir, binaryName);

  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  // Check if binary is installed locally in PATH
  try {
    const localBin = execSync(process.platform === 'win32' ? 'where bundlecheck' : 'which bundlecheck', { encoding: 'utf8', stdio: ['pipe', 'pipe', 'ignore'] }).trim().split('\n')[0];
    if (localBin && fs.existsSync(localBin)) {
      return localBin;
    }
  } catch (_) {}

  const archiveName = getArchiveName(process.platform, process.arch);
  const downloadUri = `https://github.com/${REPO}/releases/download/v${VERSION}/${archiveName}`;
  const archivePath = path.join(cacheDir, archiveName);

  process.stderr.write(`[bundlecheck] Downloading v${VERSION} binary for ${process.platform}/${process.arch}...\n`);

  try {
    await downloadUrl(downloadUri, archivePath);
    extractArchive(archivePath, cacheDir);
    if (fs.existsSync(archivePath)) {
      fs.unlinkSync(archivePath);
    }
    if (process.platform !== 'win32') {
      fs.chmodSync(binaryPath, 0o755);
    }
    return binaryPath;
  } catch (err) {
    throw new Error(`Failed to obtain bundlecheck binary: ${err.message}`);
  }
}

async function main() {
  try {
    const binaryPath = await ensureBinary();
    const result = spawnSync(binaryPath, process.argv.slice(2), {
      stdio: 'inherit'
    });
    process.exit(result.status ?? 0);
  } catch (err) {
    console.error(`\x1b[31mError:\x1b[0m ${err.message}`);
    process.exit(1);
  }
}

main();
