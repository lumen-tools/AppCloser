AppCloser - instantly closes programs when another program starts
=================================================================

Steam Deck (Desktop Mode):
  1. Extract the folder, e.g. to /home/deck/AppCloser.
  2. Right-click "AppCloser" -> Properties -> Permissions ->
     tick "Is executable" (also for "AppCloser-Settings"). Then
     double-click "AppCloser". Keep both files together.

Rules: "New rule" -> left: "Choose program…" -> "Nextcloud" (Flatpak)
       -> right: "Add trigger…" -> "RetroDECK" (Flatpak).
       Flatpak apps are detected including all subprocesses (emulators),
       no matter whether a game is launched from Steam or from RetroDECK.

Tick "Start with system": AppCloser then monitors from power-on, also in
Gaming Mode (invisible). In Desktop Mode the tray icon appears as well.

Closing the window (X) keeps it running in the tray. The settings window is
its own process and frees all its memory when closed. Click the tray icon to
reopen it.
Quit completely: tray icon -> "Quit AppCloser" (also stops Gaming Mode
monitoring until the next reboot or until AppCloser is started again).

Remove: untick "Start with system", quit, delete the folder.
(Leftover autostart entries are ignored by the system once the folder is gone.)
