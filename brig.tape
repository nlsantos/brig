Output .github/assets/brig.gif

Require ./brig

Set Shell "bash"
Set FontSize 24
Set Width 1200
Set Height 600

Type "cd ~/Garage/brig/main" Sleep 500ms  Enter Sleep 1s
Type "ls -la" Sleep 500ms  Enter Sleep 1s
Type "whoami" Sleep 500ms  Enter Sleep 1s
Type "./brig -R -v" Sleep 500ms Enter Sleep 10s
Type "ls -la" Sleep 500ms  Enter Sleep 1s
Type "whoami" Sleep 500ms  Enter Sleep 1s
Ctrl+D Sleep 500ms
Type "./brig --version" Sleep 500ms Enter Sleep 5s
