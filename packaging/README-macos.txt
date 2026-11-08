AppCloser - instantly closes programs when another program starts
=================================================================

Install:
  1. Create a folder, e.g. /Applications/AppCloser, and move AppCloser.app
     into it. Settings are stored next to the app, so deleting this folder
     later removes everything.
  2. Open AppCloser.app. macOS blocks it the first time because it is not
     notarized: open System Settings -> Privacy & Security, scroll down and
     click "Open Anyway", then confirm.

AppCloser lives in the menu bar (power symbol). Click it for the menu:
"Open Settings", "Pause Monitoring", "Quit AppCloser".

Rules: "Add Rule" -> choose the app to close (e.g. Nextcloud)
       -> "Add Trigger..." -> choose the apps that trigger it.

Notifications: macOS asks once whether AppCloser may show notifications.

Remove: untick "Start with System", quit AppCloser, delete the folder.
(A leftover login item does nothing once the app is gone.)
