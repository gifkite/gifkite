#ifndef SCK_DARWIN_H
#define SCK_DARWIN_H

#include <stdbool.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Returns true if running on macOS 12.3+ where ScreenCaptureKit is available.
bool isSCKSupported(void);

// Starts an SCK capture stream. Returns an opaque pointer to the session, or NULL on error.
void* startSCKStream(int displayIndex, int windowID, int x, int y, int w, int h, int fps, bool showCursor, uintptr_t goCtx);

// Stops an active SCK stream and releases its resources.
void stopSCKStream(void* sessionPtr);

#ifdef __cplusplus
}
#endif

#endif // SCK_DARWIN_H
