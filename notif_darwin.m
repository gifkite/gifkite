//go:build darwin

#import <Cocoa/Cocoa.h>
#include "notif_darwin.h"

@interface GifkiteNotificationDelegate : NSObject <NSUserNotificationCenterDelegate>
@end

@implementation GifkiteNotificationDelegate
- (BOOL)userNotificationCenter:(NSUserNotificationCenter *)center shouldPresentNotification:(NSUserNotification *)notification {
    return YES;
}
@end

static GifkiteNotificationDelegate *s_notifDelegate = nil;

void showNativeDarwinNotification(const char* cTitle, const char* cMessage) {
    if (!cTitle || !cMessage) return;
    NSString *title = [NSString stringWithUTF8String:cTitle];
    NSString *message = [NSString stringWithUTF8String:cMessage];

    dispatch_async(dispatch_get_main_queue(), ^{
        NSBundle *mainBundle = [NSBundle mainBundle];
        if ([mainBundle bundleIdentifier]) {
            if (!s_notifDelegate) {
                s_notifDelegate = [[GifkiteNotificationDelegate alloc] init];
                [[NSUserNotificationCenter defaultUserNotificationCenter] setDelegate:s_notifDelegate];
            }
            NSUserNotification *notif = [[NSUserNotification alloc] init];
            notif.title = title;
            notif.informativeText = message;
            notif.soundName = @"Pop";
            [[NSUserNotificationCenter defaultUserNotificationCenter] deliverNotification:notif];
        } else {
            // Unbundled dev binary or CLI fallback: route explicitly through System Events so Script Editor is never opened
            NSString *scriptSource = [NSString stringWithFormat:
                @"tell application \"System Events\" to display notification \"%@\" with title \"%@\" sound name \"Pop\"",
                [message stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""],
                [title stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""]];
            NSAppleScript *as = [[NSAppleScript alloc] initWithSource:scriptSource];
            [as executeAndReturnError:nil];
        }
    });
}

void copyNativeFileToClipboard(const char* cpath) {
    if (!cpath) return;
    NSString *path = [NSString stringWithUTF8String:cpath];
    void (^block)(void) = ^{
        NSURL *fileURL = [NSURL fileURLWithPath:path];
        NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
        [pasteboard clearContents];
        [pasteboard writeObjects:@[fileURL]];
    };

    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
}
