#import <CoreWLAN/CoreWLAN.h>
#include <stdlib.h>

char *currentSSID() {
    @autoreleasepool {
        CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
        CWInterface *iface = [client interface];
        if (!iface) {
            return NULL;
        }
        NSString *ssid = [iface ssid];
        if (!ssid) {
            return NULL;
        }
        const char *utf8 = [ssid UTF8String];
        if (!utf8) {
            return NULL;
        }
        return strdup(utf8);
    }
}
