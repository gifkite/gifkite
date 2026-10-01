#ifndef DRAG_DARWIN_H
#define DRAG_DARWIN_H

void performNativeDrag(void* nsWindowPtr, const char* cpath);
void setWindowInvisibleToCapture(void* nsWindowPtr);

#endif
