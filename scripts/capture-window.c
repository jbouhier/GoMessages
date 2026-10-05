#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

static int number(CFDictionaryRef window, const CFStringRef key, int *value) {
  CFNumberRef item = CFDictionaryGetValue(window, key);
  return item && CFGetTypeID(item) == CFNumberGetTypeID() &&
         CFNumberGetValue(item, kCFNumberIntType, value);
}

static CGWindowID selected_window(pid_t pid, int smallest) {
  CFArrayRef windows = CGWindowListCopyWindowInfo(
      kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
      kCGNullWindowID);
  if (!windows) return 0;
  CGWindowID best = 0;
  double best_area = smallest ? 1e100 : 0;
  int candidates = 0;
  for (CFIndex i = 0; i < CFArrayGetCount(windows); i++) {
    CFDictionaryRef window = CFArrayGetValueAtIndex(windows, i);
    int owner, layer, id;
    if (!number(window, kCGWindowOwnerPID, &owner) || owner != pid ||
        !number(window, kCGWindowLayer, &layer) || layer != 0 ||
        !number(window, kCGWindowNumber, &id)) continue;
    CFDictionaryRef bounds = CFDictionaryGetValue(window, kCGWindowBounds);
    CGRect rect;
    if (!bounds || !CGRectMakeWithDictionaryRepresentation(bounds, &rect)) continue;
    double area = rect.size.width * rect.size.height;
    if (area <= 100000) continue;
    candidates++;
    if (smallest ? area < best_area : area > best_area) {
      best_area = area;
      best = (CGWindowID)id;
    }
  }
  CFRelease(windows);
  if (smallest && candidates < 2) return 0;
  return best;
}

int main(int argc, char **argv) {
  if (argc < 3 || argc > 4 ||
      (argc == 4 && strcmp(argv[3], "smallest") != 0)) {
    fprintf(stderr, "Usage: capture-window <pid> <output.png> [smallest]\n");
    return 2;
  }
  pid_t pid = (pid_t)atoi(argv[1]);
  int smallest = argc == 4;
  CGWindowID id = 0;
  for (int i = 0; i < 80 && !id; i++) {
    id = selected_window(pid, smallest);
    if (!id) usleep(250000);
  }
  if (!id) {
    fprintf(stderr, "GoMessages window did not appear within 20 seconds.\n");
    return 1;
  }
  sleep(2); // Let WebKit paint the local demo.
  char id_string[24];
  snprintf(id_string, sizeof(id_string), "%u", id);
  pid_t child = fork();
  if (child == 0) {
    execl("/usr/sbin/screencapture", "screencapture", "-x", "-o", "-l",
          id_string, argv[2], (char *)NULL);
    _exit(127);
  }
  if (child < 0) {
    perror("fork");
    return 1;
  }
  int status;
  if (waitpid(child, &status, 0) < 0 || !WIFEXITED(status) ||
      WEXITSTATUS(status) != 0 || access(argv[2], F_OK) != 0) {
    fprintf(stderr, "Window capture failed. Allow Screen Recording for the terminal running this script.\n");
    return 1;
  }
  puts(argv[2]);
  return 0;
}
