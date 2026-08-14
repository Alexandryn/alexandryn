// Prototype only — simulates the role Electron's main process plays:
// spawn the Go binary as a child, read its announced port from stdout,
// and otherwise just stay alive. Plain Node child_process, not Electron
// itself — Electron's main process uses the same underlying child_process
// API, so this validates the OS-level spawn/kill mechanics (FR-1, FR-2,
// FR-9, FR-10) without needing a display to run a real Electron window.
const { spawn } = require("child_process");

const childPath = process.argv[2];
if (!childPath) {
  console.error("usage: node parent.js <path-to-go-binary>");
  process.exit(1);
}

const child = spawn(childPath, [], { stdio: ["ignore", "pipe", "inherit"] });

console.log(`PARENT_PID=${process.pid}`);
console.log(`CHILD_PID=${child.pid}`);

let announced = false;
child.stdout.on("data", (buf) => {
  const line = buf.toString();
  process.stdout.write(line);
  if (!announced && line.startsWith("PORT=")) {
    announced = true;
  }
});

child.on("exit", (code, signal) => {
  console.log(`CHILD_EXITED code=${code} signal=${signal}`);
});

// FR-2 / FR-9: normal quit path — parent forwards SIGTERM to the child, then
// exits itself once the child is gone.
process.on("SIGTERM", () => {
  console.log("PARENT_GOT_SIGTERM, forwarding to child");
  child.kill("SIGTERM");
  child.on("exit", () => process.exit(0));
});

// Keep the parent alive indefinitely, same as Electron's main process would
// stay alive for the life of the app.
setInterval(() => {}, 1000 * 60 * 60);
