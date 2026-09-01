export function Snapshot() {
  return window.go.main.App.Snapshot();
}
export function SendTo(stableID, paths) {
  return window.go.main.App.SendTo(stableID, paths);
}
export function SelectFiles() {
  return window.go.main.App.SelectFiles();
}
export function SelectRecvDir() {
  return window.go.main.App.SelectRecvDir();
}
export function DefaultRecvDir() {
  return window.go.main.App.DefaultRecvDir();
}
export function ReceiveTo(dir, conflict) {
  return window.go.main.App.ReceiveTo(dir, conflict);
}
export function PairingJSON() {
  return window.go.main.App.PairingJSON();
}
export function PairingQR() {
  return window.go.main.App.PairingQR();
}
export function Platform() {
  return window.go.main.App.Platform();
}
