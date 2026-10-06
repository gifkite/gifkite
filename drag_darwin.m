//go:build darwin

#import <Cocoa/Cocoa.h>
#import "drag_darwin.h"

extern void onNativeDragEnded(int op);

@interface GifkiteDragSource : NSObject <NSDraggingSource>
@end

@implementation GifkiteDragSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
    return NSDragOperationCopy;
}

- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)screenPoint operation:(NSDragOperation)operation {
    onNativeDragEnded((int)operation);
}
@end

static GifkiteDragSource *s_dragSource = nil;

void performNativeDrag(void* nsWindowPtr, const char* cpath) {
    if (!nsWindowPtr || !cpath) return;
    NSString *path = [NSString stringWithUTF8String:cpath];

    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = (__bridge NSWindow*)nsWindowPtr;
        NSView *view = [window contentView];
        if (!view) return;

        if (!s_dragSource) {
            s_dragSource = [[GifkiteDragSource alloc] init];
        }

        NSURL *fileURL = [NSURL fileURLWithPath:path];
        NSDraggingItem *dragItem = [[NSDraggingItem alloc] initWithPasteboardWriter:fileURL];

        NSImage *icon = [[NSImage alloc] initWithContentsOfFile:path];
        if (!icon) {
            icon = [[NSWorkspace sharedWorkspace] iconForFile:path];
        }

        NSSize originalSize = [icon size];
        CGFloat maxDim = 120.0;
        CGFloat targetW = maxDim;
        CGFloat targetH = maxDim;
        if (originalSize.width > 0 && originalSize.height > 0) {
            CGFloat aspect = originalSize.width / originalSize.height;
            if (aspect > 1.0) {
                targetW = maxDim;
                targetH = maxDim / aspect;
            } else {
                targetH = maxDim;
                targetW = maxDim * aspect;
            }
        }
        [icon setSize:NSMakeSize(targetW, targetH)];

        NSPoint mouseLoc = [NSEvent mouseLocation];
        NSPoint locInWindow = [window convertPointFromScreen:mouseLoc];
        NSRect frame = NSMakeRect(locInWindow.x - targetW/2.0, locInWindow.y - targetH/2.0, targetW, targetH);
        [dragItem setDraggingFrame:frame contents:icon];

        NSEvent *event = [NSApp currentEvent];
        if (!event || (event.type != NSEventTypeLeftMouseDown && event.type != NSEventTypeLeftMouseDragged)) {
            event = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDown
                                       location:locInWindow
                                  modifierFlags:0
                                      timestamp:[[NSProcessInfo processInfo] systemUptime]
                                   windowNumber:window.windowNumber
                                        context:nil
                                    eventNumber:0
                                     clickCount:1
                                      pressure:1.0];
        }

        [view beginDraggingSessionWithItems:@[dragItem] event:event source:s_dragSource];
    });
}

void setWindowInvisibleToCapture(void* nsWindowPtr) {
    if (!nsWindowPtr) return;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = (__bridge NSWindow*)nsWindowPtr;
        if ([window respondsToSelector:@selector(setSharingType:)]) {
            [window setSharingType:NSWindowSharingNone];
        }
    });
}

void setWindowTransparent(void* nsWindowPtr) {
    if (!nsWindowPtr) return;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *window = (__bridge NSWindow*)nsWindowPtr;
        [window setOpaque:NO];
        [window setBackgroundColor:[NSColor clearColor]];
        [window setHasShadow:NO];

        NSView *contentView = [window contentView];
        if (contentView) {
            [contentView setWantsLayer:YES];
            contentView.layer.backgroundColor = [[NSColor clearColor] CGColor];

            NSMutableArray *views = [NSMutableArray arrayWithObject:contentView];
            while ([views count] > 0) {
                NSView *v = [views firstObject];
                [views removeObjectAtIndex:0];

                @try {
                    [v setValue:@NO forKey:@"drawsBackground"];
                } @catch (NSException *e) {}

                if ([v respondsToSelector:@selector(setUnderPageBackgroundColor:)]) {
                    @try {
                        [v performSelector:@selector(setUnderPageBackgroundColor:) withObject:[NSColor clearColor]];
                    } @catch (NSException *e) {}
                }

                [views addObjectsFromArray:[v subviews]];
            }
        }
    });
}

