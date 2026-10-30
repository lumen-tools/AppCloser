AppCloser - instantly closes programs when another program starts
=================================================================

Start:   Double-click AppCloser.exe (the small tray program; it opens
         AppCloser-Settings.exe for the window). Keep both files together. On first launch Windows SmartScreen may warn
         -> "More info" -> "Run anyway".

Rules:   "New rule" -> left: "Choose program…" (e.g. nextcloud.exe)
         -> right: "Add trigger…" (e.g. retrobat.exe, emulationstation.exe).
         Tip: start the program first so it shows up in the list.

Closing the window (X) keeps AppCloser running in the tray (blue icon).
The settings window is its own small process and frees all its memory when
closed; the tray part needs only a few MB. Left-click the tray icon to reopen.
Quit completely: right-click the tray icon -> "Quit AppCloser"
(or the "Quit AppCloser" button in the window). Nothing keeps running afterwards.

Remove:  Untick "Start with system", quit, delete the folder.
         All settings and caches live inside this folder.
