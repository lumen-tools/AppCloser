package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#include <stdlib.h>

// Shows banners even while AppCloser counts as the active app.
@interface ACNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end
@implementation ACNotifyDelegate
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
	completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList);
}
@end

static ACNotifyDelegate *acDelegate;

static void ac_notify(const char *title, const char *body) {
	// The notification center needs an app bundle; without one it would crash.
	if ([[NSBundle mainBundle] bundleIdentifier] == nil) return;
	NSString *t = [NSString stringWithUTF8String:title];
	NSString *b = [NSString stringWithUTF8String:body];
	UNUserNotificationCenter *c = [UNUserNotificationCenter currentNotificationCenter];
	if (acDelegate == nil) {
		acDelegate = [ACNotifyDelegate new];
		c.delegate = acDelegate;
	}
	[c requestAuthorizationWithOptions:UNAuthorizationOptionAlert
	                 completionHandler:^(BOOL granted, NSError *error) {
		if (!granted) return;
		UNMutableNotificationContent *content = [UNMutableNotificationContent new];
		content.title = t;
		content.body = b;
		UNNotificationRequest *r = [UNNotificationRequest requestWithIdentifier:[[NSUUID UUID] UUIDString]
		                                                                content:content trigger:nil];
		[c addNotificationRequest:r withCompletionHandler:nil];
	}];
}
*/
import "C"

import "unsafe"

// sendNotification shows a notification from AppCloser itself (Notification
// Center); macOS asks once for permission.
func sendNotification(title, body string) {
	t, b := C.CString(title), C.CString(body)
	defer C.free(unsafe.Pointer(t))
	defer C.free(unsafe.Pointer(b))
	C.ac_notify(t, b)
}
