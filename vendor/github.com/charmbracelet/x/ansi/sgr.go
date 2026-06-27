package ansi

// SelectGraphicRendition (SGR) is a command that sets display attributes.
//
// Default is 0.
//
//	CSI Ps ; Ps ... m
//
// See: https://vt100.net/docs/vt510-rm/SGR.html
func SelectGraphicRendition(ps ...Attr) string {
	if len(ps) == 0 {
		return ResetStyle
	}

	return NewStyle(ps...).String()
}

// SGR is an alias for [SelectGraphicRendition].
func SGR(ps ...Attr) string {
	return SelectGraphicRendition(ps...)
}

var attrStrings = map[int]string{
	AttrReset:                        attrReset,
	AttrBold:                         attrBold,
	AttrFaint:                        attrFaint,
	AttrItalic:                       attrItalic,
	AttrUnderline:                    attrUnderline,
	AttrBlink:                        attrBlink,
	AttrRapidBlink:                   attrRapidBlink,
	AttrReverse:                      attrReverse,
	AttrConceal:                      attrConceal,
	AttrStrikethrough:                attrStrikethrough,
	AttrNormalIntensity:              attrNormalIntensity,
	AttrNoItalic:                     attrNoItalic,
	AttrNoUnderline:                  attrNoUnderline,
	AttrNoBlink:                      attrNoBlink,
	AttrNoReverse:                    attrNoReverse,
	AttrNoConceal:                    attrNoConceal,
	AttrNoStrikethrough:              attrNoStrikethrough,
	AttrBlackForegroundColor:         attrBlackForegroundColor,
	AttrRedForegroundColor:           attrRedForegroundColor,
	AttrGreenForegroundColor:         attrGreenForegroundColor,
	AttrYellowForegroundColor:        attrYellowForegroundColor,
	AttrBlueForegroundColor:          attrBlueForegroundColor,
	AttrMagentaForegroundColor:       attrMagentaForegroundColor,
	AttrCyanForegroundColor:          attrCyanForegroundColor,
	AttrWhiteForegroundColor:         attrWhiteForegroundColor,
	AttrExtendedForegroundColor:      attrExtendedForegroundColor,
	AttrDefaultForegroundColor:       attrDefaultForegroundColor,
	AttrBlackBackgroundColor:         attrBlackBackgroundColor,
	AttrRedBackgroundColor:           attrRedBackgroundColor,
	AttrGreenBackgroundColor:         attrGreenBackgroundColor,
	AttrYellowBackgroundColor:        attrYellowBackgroundColor,
	AttrBlueBackgroundColor:          attrBlueBackgroundColor,
	AttrMagentaBackgroundColor:       attrMagentaBackgroundColor,
	AttrCyanBackgroundColor:          attrCyanBackgroundColor,
	AttrWhiteBackgroundColor:         attrWhiteBackgroundColor,
	AttrExtendedBackgroundColor:      attrExtendedBackgroundColor,
	AttrDefaultBackgroundColor:       attrDefaultBackgroundColor,
	AttrExtendedUnderlineColor:       attrExtendedUnderlineColor,
	AttrDefaultUnderlineColor:        attrDefaultUnderlineColor,
	AttrBrightBlackForegroundColor:   attrBrightBlackForegroundColor,
	AttrBrightRedForegroundColor:     attrBrightRedForegroundColor,
	AttrBrightGreenForegroundColor:   attrBrightGreenForegroundColor,
	AttrBrightYellowForegroundColor:  attrBrightYellowForegroundColor,
	AttrBrightBlueForegroundColor:    attrBrightBlueForegroundColor,
	AttrBrightMagentaForegroundColor: attrBrightMagentaForegroundColor,
	AttrBrightCyanForegroundColor:    attrBrightCyanForegroundColor,
	AttrBrightWhiteForegroundColor:   attrBrightWhiteForegroundColor,
	AttrBrightBlackBackgroundColor:   attrBrightBlackBackgroundColor,
	AttrBrightRedBackgroundColor:     attrBrightRedBackgroundColor,
	AttrBrightGreenBackgroundColor:   attrBrightGreenBackgroundColor,
	AttrBrightYellowBackgroundColor:  attrBrightYellowBackgroundColor,
	AttrBrightBlueBackgroundColor:    attrBrightBlueBackgroundColor,
	AttrBrightMagentaBackgroundColor: attrBrightMagentaBackgroundColor,
	AttrBrightCyanBackgroundColor:    attrBrightCyanBackgroundColor,
	AttrBrightWhiteBackgroundColor:   attrBrightWhiteBackgroundColor,
}
