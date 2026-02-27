Output ./docs/demo.gif

Require tug
Require curl

Set Shell "zsh"
Set FontSize 16
Set Width 960
Set Height 540
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set PlaybackSpeed 0.5

# Start app-a
Type "cd examples/app-a"
Enter
Sleep 500ms

Type "tug up -d"
Enter
Sleep 1s

Type "tug ps"
Enter
Sleep 2s

# Start app-b
Type "cd ../app-b"
Enter
Sleep 500ms

Type "tug up -d"
Enter
Sleep 1s

Type "tug ps"
Enter
Sleep 2s

# Prove both are accessible
Type "curl -s http://api.app-a.localhost | head -5"
Enter
Sleep 1s

Type "curl -s http://api.app-b.localhost | head -5"
Enter
Sleep 2s
