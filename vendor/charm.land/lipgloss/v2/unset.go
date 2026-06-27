package lipgloss

// unset unsets a property from a style.
func (s *Style) unset(key propKey) {
	s.props = s.props.unset(key)
}

// UnsetBold removes the bold style rule, if set.
func (s Style) UnsetBold() Style {
	s.unset(boldKey)
	return s
}

// UnsetItalic removes the italic style rule, if set.
func (s Style) UnsetItalic() Style {
	s.unset(italicKey)
	return s
}

// UnsetUnderline removes the underline style rule, if set.
func (s Style) UnsetUnderline() Style {
	return s.Underline(false)
}

// UnsetStrikethrough removes the strikethrough style rule, if set.
func (s Style) UnsetStrikethrough() Style {
	s.unset(strikethroughKey)
	return s
}

// UnsetReverse removes the reverse style rule, if set.
func (s Style) UnsetReverse() Style {
	s.unset(reverseKey)
	return s
}

// UnsetBlink removes the blink style rule, if set.
func (s Style) UnsetBlink() Style {
	s.unset(blinkKey)
	return s
}

// UnsetFaint removes the faint style rule, if set.
func (s Style) UnsetFaint() Style {
	s.unset(faintKey)
	return s
}

// UnsetForeground removes the foreground style rule, if set.
func (s Style) UnsetForeground() Style {
	s.unset(foregroundKey)
	return s
}

// UnsetBackground removes the background style rule, if set.
func (s Style) UnsetBackground() Style {
	s.unset(backgroundKey)
	return s
}

// UnsetWidth removes the width style rule, if set.
func (s Style) UnsetWidth() Style {
	s.unset(widthKey)
	return s
}

// UnsetHeight removes the height style rule, if set.
func (s Style) UnsetHeight() Style {
	s.unset(heightKey)
	return s
}

// UnsetAlign removes the horizontal and vertical text alignment style rule, if set.
func (s Style) UnsetAlign() Style {
	s.unset(alignHorizontalKey)
	s.unset(alignVerticalKey)
	return s
}

// UnsetAlignHorizontal removes the horizontal text alignment style rule, if set.
func (s Style) UnsetAlignHorizontal() Style {
	s.unset(alignHorizontalKey)
	return s
}

// UnsetAlignVertical removes the vertical text alignment style rule, if set.
func (s Style) UnsetAlignVertical() Style {
	s.unset(alignVerticalKey)
	return s
}

// UnsetPadding removes all padding style rules.
func (s Style) UnsetPadding() Style {
	s.unset(paddingLeftKey)
	s.unset(paddingRightKey)
	s.unset(paddingTopKey)
	s.unset(paddingBottomKey)
	s.unset(paddingCharKey)
	return s
}

// UnsetPaddingChar removes the padding character style rule, if set.
func (s Style) UnsetPaddingChar() Style {
	s.unset(paddingCharKey)
	return s
}

// UnsetPaddingLeft removes the left padding style rule, if set.
func (s Style) UnsetPaddingLeft() Style {
	s.unset(paddingLeftKey)
	return s
}

// UnsetPaddingRight removes the right padding style rule, if set.
func (s Style) UnsetPaddingRight() Style {
	s.unset(paddingRightKey)
	return s
}

// UnsetPaddingTop removes the top padding style rule, if set.
func (s Style) UnsetPaddingTop() Style {
	s.unset(paddingTopKey)
	return s
}

// UnsetPaddingBottom removes the bottom padding style rule, if set.
func (s Style) UnsetPaddingBottom() Style {
	s.unset(paddingBottomKey)
	return s
}

// UnsetColorWhitespace removes the rule for coloring padding, if set.
func (s Style) UnsetColorWhitespace() Style {
	s.unset(colorWhitespaceKey)
	return s
}

// UnsetMargins removes all margin style rules.
func (s Style) UnsetMargins() Style {
	s.unset(marginLeftKey)
	s.unset(marginRightKey)
	s.unset(marginTopKey)
	s.unset(marginBottomKey)
	return s
}

// UnsetMarginLeft removes the left margin style rule, if set.
func (s Style) UnsetMarginLeft() Style {
	s.unset(marginLeftKey)
	return s
}

// UnsetMarginRight removes the right margin style rule, if set.
func (s Style) UnsetMarginRight() Style {
	s.unset(marginRightKey)
	return s
}

// UnsetMarginTop removes the top margin style rule, if set.
func (s Style) UnsetMarginTop() Style {
	s.unset(marginTopKey)
	return s
}

// UnsetMarginBottom removes the bottom margin style rule, if set.
func (s Style) UnsetMarginBottom() Style {
	s.unset(marginBottomKey)
	return s
}

// UnsetMarginBackground removes the margin's background color. Note that the
// margin's background color can be set from the background color of another
// style during inheritance.
func (s Style) UnsetMarginBackground() Style {
	s.unset(marginBackgroundKey)
	return s
}

// UnsetBorderStyle removes the border style rule, if set.
func (s Style) UnsetBorderStyle() Style {
	s.unset(borderStyleKey)
	return s
}

// UnsetBorderTop removes the border top style rule, if set.
func (s Style) UnsetBorderTop() Style {
	s.unset(borderTopKey)
	return s
}

// UnsetBorderRight removes the border right style rule, if set.
func (s Style) UnsetBorderRight() Style {
	s.unset(borderRightKey)
	return s
}

// UnsetBorderBottom removes the border bottom style rule, if set.
func (s Style) UnsetBorderBottom() Style {
	s.unset(borderBottomKey)
	return s
}

// UnsetBorderLeft removes the border left style rule, if set.
func (s Style) UnsetBorderLeft() Style {
	s.unset(borderLeftKey)
	return s
}

// UnsetBorderForeground removes all border foreground color styles, if set.
func (s Style) UnsetBorderForeground() Style {
	s.unset(borderTopForegroundKey)
	s.unset(borderRightForegroundKey)
	s.unset(borderBottomForegroundKey)
	s.unset(borderLeftForegroundKey)
	return s
}

// UnsetBorderTopForeground removes the top border foreground color rule,
// if set.
func (s Style) UnsetBorderTopForeground() Style {
	s.unset(borderTopForegroundKey)
	return s
}

// UnsetBorderRightForeground removes the right border foreground color rule,
// if set.
func (s Style) UnsetBorderRightForeground() Style {
	s.unset(borderRightForegroundKey)
	return s
}

// UnsetBorderBottomForeground removes the bottom border foreground color
// rule, if set.
func (s Style) UnsetBorderBottomForeground() Style {
	s.unset(borderBottomForegroundKey)
	return s
}

// UnsetBorderLeftForeground removes the left border foreground color rule,
// if set.
func (s Style) UnsetBorderLeftForeground() Style {
	s.unset(borderLeftForegroundKey)
	return s
}

// UnsetBorderForegroundBlend removes the border blend foreground color rules,
// if set.
func (s Style) UnsetBorderForegroundBlend() Style {
	s.unset(borderForegroundBlendKey)
	return s
}

// UnsetBorderForegroundBlendOffset removes the border blend offset style rule,
// if set.
func (s Style) UnsetBorderForegroundBlendOffset() Style {
	s.unset(borderForegroundBlendOffsetKey)
	return s
}

// UnsetBorderBackground removes all border background color styles, if
// set.
func (s Style) UnsetBorderBackground() Style {
	s.unset(borderTopBackgroundKey)
	s.unset(borderRightBackgroundKey)
	s.unset(borderBottomBackgroundKey)
	s.unset(borderLeftBackgroundKey)
	return s
}

// UnsetBorderTopBackgroundColor removes the top border background color rule,
// if set.
//
// Deprecated: This function simply calls Style.UnsetBorderTopBackground.
func (s Style) UnsetBorderTopBackgroundColor() Style {
	return s.UnsetBorderTopBackground()
}

// UnsetBorderTopBackground removes the top border background color rule,
// if set.
func (s Style) UnsetBorderTopBackground() Style {
	s.unset(borderTopBackgroundKey)
	return s
}

// UnsetBorderRightBackground removes the right border background color
// rule, if set.
func (s Style) UnsetBorderRightBackground() Style {
	s.unset(borderRightBackgroundKey)
	return s
}

// UnsetBorderBottomBackground removes the bottom border background color
// rule, if set.
func (s Style) UnsetBorderBottomBackground() Style {
	s.unset(borderBottomBackgroundKey)
	return s
}

// UnsetBorderLeftBackground removes the left border color rule, if set.
func (s Style) UnsetBorderLeftBackground() Style {
	s.unset(borderLeftBackgroundKey)
	return s
}

// UnsetInline removes the inline style rule, if set.
func (s Style) UnsetInline() Style {
	s.unset(inlineKey)
	return s
}

// UnsetMaxWidth removes the max width style rule, if set.
func (s Style) UnsetMaxWidth() Style {
	s.unset(maxWidthKey)
	return s
}

// UnsetMaxHeight removes the max height style rule, if set.
func (s Style) UnsetMaxHeight() Style {
	s.unset(maxHeightKey)
	return s
}

// UnsetTabWidth removes the tab width style rule, if set.
func (s Style) UnsetTabWidth() Style {
	s.unset(tabWidthKey)
	return s
}

// UnsetUnderlineSpaces removes the value set by UnderlineSpaces.
func (s Style) UnsetUnderlineSpaces() Style {
	s.unset(underlineSpacesKey)
	return s
}

// UnsetStrikethroughSpaces removes the value set by StrikethroughSpaces.
func (s Style) UnsetStrikethroughSpaces() Style {
	s.unset(strikethroughSpacesKey)
	return s
}

// UnsetTransform removes the value set by Transform.
func (s Style) UnsetTransform() Style {
	s.unset(transformKey)
	return s
}

// UnsetHyperlink removes the value set by Hyperlink.
func (s Style) UnsetHyperlink() Style {
	s.unset(linkKey)
	s.unset(linkParamsKey)
	s.link, s.linkParams = "", "" // save memory
	return s
}

// UnsetString sets the underlying string value to the empty string.
func (s Style) UnsetString() Style {
	s.value = ""
	return s
}
