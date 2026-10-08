#import <Cocoa/Cocoa.h>

static const CGFloat kCell = 30, kRow = 28, kDot = 24, kPad = 12, kTop = 4, kHeader = 22, kLegend = 28;

// MenuCalendarView draws one month as a Monday-first grid of day numbers.
// cells holds one character per grid cell; see menucal.Set for the alphabet.
@interface MenuCalendarView : NSView
@property (copy) NSString *cells;
@end

static NSColor *hex(unsigned rgb) {
  return [NSColor colorWithSRGBRed:((rgb >> 16) & 0xFF) / 255.0
                             green:((rgb >> 8) & 0xFF) / 255.0
                              blue:(rgb & 0xFF) / 255.0
                             alpha:1];
}

// Soft pastel palette. The fills are light in both light and dark mode, so the
// day number on them is always dark (see drawRect:).
static NSColor *fillColor(unichar c) {
  switch (c) {
    case 'p': return hex(0xBEDAE3); // attended
    case 'h': return hex(0xC9E4CA); // holiday
    case 'l': return hex(0xFED5CF); // leave
  }
  return nil;
}

static NSColor *absentColor(void) { return hex(0xD3C7E6); }

static void drawText(NSString *s, NSRect r, NSColor *color, CGFloat size, NSTextAlignment align) {
  NSMutableParagraphStyle *style = [[NSMutableParagraphStyle alloc] init];
  style.alignment = align;
  NSDictionary *attrs = @{
    NSFontAttributeName: [NSFont monospacedDigitSystemFontOfSize:size weight:NSFontWeightRegular],
    NSForegroundColorAttributeName: color,
    NSParagraphStyleAttributeName: style,
  };
  CGFloat h = [s sizeWithAttributes:attrs].height;
  r.origin.y += (r.size.height - h) / 2;
  r.size.height = h;
  [s drawInRect:r withAttributes:attrs];
}

// drawDot draws the marker for a cell state: filled for present, holiday and leave,
// a lavender ring for absent.
static void drawDot(unichar c, NSRect r) {
  NSColor *fill = fillColor(c);
  if (fill) {
    [fill setFill];
    [[NSBezierPath bezierPathWithOvalInRect:r] fill];
  } else if (c == 'a') {
    NSBezierPath *ring = [NSBezierPath bezierPathWithOvalInRect:NSInsetRect(r, 0.75, 0.75)];
    ring.lineWidth = 2;
    [absentColor() setStroke];
    [ring stroke];
  }
}

@implementation MenuCalendarView

- (BOOL)isFlipped { return YES; }

- (NSSize)fittingSize {
  NSUInteger rows = (self.cells.length + 6) / 7;
  return NSMakeSize(2 * kPad + 7 * kCell, kTop + kHeader + rows * kRow + kLegend);
}

- (void)drawRect:(NSRect)dirtyRect {
  NSArray<NSString *> *weekdays = @[@"Mo", @"Tu", @"We", @"Th", @"Fr", @"Sa", @"Su"];
  for (NSUInteger i = 0; i < 7; i++) {
    drawText(weekdays[i], NSMakeRect(kPad + i * kCell, kTop, kCell, kHeader),
             NSColor.secondaryLabelColor, 11, NSTextAlignmentCenter);
  }

  int day = 0;
  for (NSUInteger i = 0; i < self.cells.length; i++) {
    unichar c = [self.cells characterAtIndex:i];
    if (c == '.') continue; // padding before the 1st or after the last day
    day++;
    NSRect cell = NSMakeRect(kPad + (i % 7) * kCell, kTop + kHeader + (i / 7) * kRow, kCell, kRow);
    drawDot(c, NSInsetRect(cell, (kCell - kDot) / 2, (kRow - kDot) / 2));

    NSColor *text = NSColor.labelColor;
    if (fillColor(c)) text = hex(0x3A3A3C);
    else if (c == 'w') text = NSColor.tertiaryLabelColor;
    else if (c == 'n') text = NSColor.secondaryLabelColor;
    drawText([NSString stringWithFormat:@"%d", day], cell, text, 12, NSTextAlignmentCenter);
  }

  NSArray<NSString *> *legend = @[@"pPresent", @"aAbsent", @"hHoliday", @"lLeave"];
  CGFloat y = kTop + kHeader + ((self.cells.length + 6) / 7) * kRow;
  CGFloat w = 7 * kCell / legend.count;
  for (NSUInteger i = 0; i < legend.count; i++) {
    CGFloat x = kPad + i * w;
    drawDot([legend[i] characterAtIndex:0], NSMakeRect(x + 2, y + (kLegend - 8) / 2, 8, 8));
    drawText([legend[i] substringFromIndex:1], NSMakeRect(x + 14, y, w - 14, kLegend),
             NSColor.secondaryLabelColor, 10, NSTextAlignmentLeft);
  }
}

@end

static NSMenuItem *findByTitle(NSMenu *menu, NSString *title) {
  for (NSMenuItem *item in menu.itemArray) {
    if ([item.title isEqualToString:title]) return item;
    NSMenuItem *found = item.submenu ? findByTitle(item.submenu, title) : nil;
    if (found) return found;
  }
  return nil;
}

void setMenuCalendar(const char *placeholder, const char *cells) {
  NSString *title = [NSString stringWithUTF8String:placeholder];
  NSString *state = [NSString stringWithUTF8String:cells];
  dispatch_async(dispatch_get_main_queue(), ^{
    // systray keeps its NSMenu in a private ivar of the app delegate and has no
    // API for custom item views, so reach it by name. If a systray upgrade renames
    // it, the placeholder title stays visible instead of the calendar.
    NSMenu *menu = nil;
    @try {
      menu = [(NSObject *)NSApp.delegate valueForKey:@"menu"];
    } @catch (NSException *e) {
      return;
    }
    NSMenuItem *item = findByTitle(menu, title);
    if (!item) return;
    MenuCalendarView *view = (MenuCalendarView *)item.view;
    if (!view) {
      view = [[MenuCalendarView alloc] init];
      item.view = view;
    }
    view.cells = state;
    [view setFrameSize:view.fittingSize];
    [view setNeedsDisplay:YES];
  });
}
