#import <Cocoa/Cocoa.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreMedia/CoreMedia.h>
#import <CoreVideo/CoreVideo.h>
#include "sck_darwin.h"

extern void sckFrameCallback(uintptr_t ctx, const void* baseAddress, int width, int height, int bytesPerRow, int64_t ptsNs);

@interface SCKSession : NSObject <SCStreamOutput, SCStreamDelegate>
@property (nonatomic, strong) SCStream *stream;
@property (nonatomic, assign) uintptr_t goCtx;
@property (nonatomic, assign) BOOL isRunning;
@property (nonatomic, strong) dispatch_queue_t queue;
@end

@implementation SCKSession

- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeScreen || !self.isRunning) {
        return;
    }

    CFArrayRef attachments = CMSampleBufferGetSampleAttachmentsArray(sampleBuffer, false);
    if (attachments && CFArrayGetCount(attachments) > 0) {
        CFDictionaryRef dict = (CFDictionaryRef)CFArrayGetValueAtIndex(attachments, 0);
        if (dict) {
            CFNumberRef statusNum = (CFNumberRef)CFDictionaryGetValue(dict, SCStreamFrameInfoStatus);
            if (statusNum) {
                NSInteger status = 0;
                CFNumberGetValue(statusNum, kCFNumberNSIntegerType, &status);
                if (status != SCFrameStatusComplete && status != SCFrameStatusStarted) {
                    return; // skip blank / suspended / idle frames
                }
            }
        }
    }

    CVImageBufferRef imgBuf = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!imgBuf) return;

    if (CVPixelBufferLockBaseAddress(imgBuf, kCVPixelBufferLock_ReadOnly) == kCVReturnSuccess) {
        void *base = CVPixelBufferGetBaseAddress(imgBuf);
        int w = (int)CVPixelBufferGetWidth(imgBuf);
        int h = (int)CVPixelBufferGetHeight(imgBuf);
        int stride = (int)CVPixelBufferGetBytesPerRow(imgBuf);
        CMTime pts = CMSampleBufferGetPresentationTimeStamp(sampleBuffer);
        int64_t ptsNs = (int64_t)(CMTimeGetSeconds(pts) * 1e9);

        if (base && w > 0 && h > 0) {
            sckFrameCallback(self.goCtx, base, w, h, stride, ptsNs);
        }
        CVPixelBufferUnlockBaseAddress(imgBuf, kCVPixelBufferLock_ReadOnly);
    }
}

- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
    self.isRunning = NO;
}

@end

bool isSCKSupported(void) {
    if (@available(macOS 12.3, *)) {
        return true;
    }
    return false;
}

void* startSCKStream(int displayIndex, int windowID, int x, int y, int w, int h, int fps, bool showCursor, uintptr_t goCtx) {
    if (!isSCKSupported()) {
        return NULL;
    }

    @autoreleasepool {
        [NSApplication sharedApplication];

        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block SCShareableContent *content = nil;
        __block NSError *contentErr = nil;

        [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *shareableContent, NSError *error) {
            content = shareableContent;
            contentErr = error;
            dispatch_semaphore_signal(sem);
        }];

        if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC)) != 0 || contentErr || !content) {
            return NULL;
        }

        SCContentFilter *filter = nil;
        SCStreamConfiguration *config = [[SCStreamConfiguration alloc] init];

        if (windowID > 0) {
            SCWindow *targetWin = nil;
            for (SCWindow *win in content.windows) {
                if (win.windowID == (CGWindowID)windowID) {
                    targetWin = win;
                    break;
                }
            }
            if (targetWin) {
                // True desktop-independent window isolation!
                filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:targetWin];
                config.width = MAX((size_t)16, (size_t)targetWin.frame.size.width);
                config.height = MAX((size_t)16, (size_t)targetWin.frame.size.height);
                config.scalesToFit = NO;
            }
        }

        if (!filter) {
            // Display or cropped region capture
            SCDisplay *targetDisp = nil;
            if (displayIndex >= 0 && displayIndex < (int)content.displays.count) {
                targetDisp = content.displays[displayIndex];
            } else if (content.displays.count > 0) {
                targetDisp = content.displays[0];
            }
            if (!targetDisp) {
                return NULL;
            }

            // Exclude our own app's windows from screen recording
            pid_t myPid = getpid();
            NSMutableArray<SCWindow *> *excluded = [NSMutableArray array];
            for (SCWindow *win in content.windows) {
                if (win.owningApplication.processID == myPid) {
                    [excluded addObject:win];
                }
            }

            filter = [[SCContentFilter alloc] initWithDisplay:targetDisp excludingWindows:excluded];
            if (w > 0 && h > 0) {
                config.sourceRect = CGRectMake(x, y, w, h);
                config.width = (size_t)w;
                config.height = (size_t)h;
            } else {
                config.width = (size_t)targetDisp.width;
                config.height = (size_t)targetDisp.height;
            }
            config.scalesToFit = NO;
        }

        config.minimumFrameInterval = CMTimeMake(1, fps > 0 ? fps : 30);
        config.queueDepth = 5;
        config.showsCursor = showCursor ? YES : NO;
        config.pixelFormat = kCVPixelFormatType_32BGRA;

        SCKSession *session = [[SCKSession alloc] init];
        session.goCtx = goCtx;
        session.isRunning = YES;
        session.queue = dispatch_queue_create("com.gifkite.sck", DISPATCH_QUEUE_SERIAL);

        SCStream *stream = [[SCStream alloc] initWithFilter:filter configuration:config delegate:session];
        session.stream = stream;

        NSError *outErr = nil;
        [stream addStreamOutput:session type:SCStreamOutputTypeScreen sampleHandlerQueue:session.queue error:&outErr];
        if (outErr) {
            return NULL;
        }

        dispatch_semaphore_t startSem = dispatch_semaphore_create(0);
        __block NSError *startErr = nil;
        [stream startCaptureWithCompletionHandler:^(NSError *error) {
            startErr = error;
            dispatch_semaphore_signal(startSem);
        }];

        if (dispatch_semaphore_wait(startSem, dispatch_time(DISPATCH_TIME_NOW, 3 * NSEC_PER_SEC)) != 0 || startErr) {
            return NULL;
        }

        return (__bridge_retained void *)session;
    }
}

void stopSCKStream(void* sessionPtr) {
    if (!sessionPtr) return;

    @autoreleasepool {
        SCKSession *session = (__bridge_transfer SCKSession *)sessionPtr;
        session.isRunning = NO;

        if (session.stream) {
            dispatch_semaphore_t sem = dispatch_semaphore_create(0);
            [session.stream stopCaptureWithCompletionHandler:^(NSError *error) {
                dispatch_semaphore_signal(sem);
            }];
            dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC));
            session.stream = nil;
        }
        session.queue = nil;
    }
}
