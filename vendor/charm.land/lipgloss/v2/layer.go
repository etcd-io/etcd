package lipgloss

import (
	"fmt"
	"image"
	"slices"

	uv "github.com/charmbracelet/ultraviolet"
)

// Layer represents a visual layer with content and positioning. It's a pure
// data structure that defines the layer hierarchy without any computation.
type Layer struct {
	id            string
	content       string
	width, height int
	x, y, z       int
	layers        []*Layer
}

// NewLayer creates a new [Layer] with the given content and optional child layers.
func NewLayer(content string, layers ...*Layer) *Layer {
	l := &Layer{
		content: content,
	}
	l.AddLayers(layers...)
	return l
}

// GetContent returns the content of the Layer.
func (l *Layer) GetContent() string {
	return l.content
}

// Width returns the width of the Layer.
func (l *Layer) Width() int {
	return l.width
}

// Height returns the height of the Layer.
func (l *Layer) Height() int {
	return l.height
}

// GetID returns the ID of the Layer.
func (l *Layer) GetID() string {
	return l.id
}

// ID sets the ID of the Layer.
func (l *Layer) ID(id string) *Layer {
	l.id = id
	return l
}

// X sets the x-coordinate of the Layer relative to its parent.
func (l *Layer) X(x int) *Layer {
	l.x = x
	return l
}

// Y sets the y-coordinate of the Layer relative to its parent.
func (l *Layer) Y(y int) *Layer {
	l.y = y
	return l
}

// Z sets the z-index of the Layer relative to its parent.
func (l *Layer) Z(z int) *Layer {
	l.z = z
	return l
}

// GetX returns the x-coordinate of the Layer relative to its parent.
func (l *Layer) GetX() int {
	return l.x
}

// GetY returns the y-coordinate of the Layer relative to its parent.
func (l *Layer) GetY() int {
	return l.y
}

// GetZ returns the z-index of the Layer relative to its parent.
func (l *Layer) GetZ() int {
	return l.z
}

// AddLayers adds child layers to the Layer.
func (l *Layer) AddLayers(layers ...*Layer) *Layer {
	for i, layer := range layers {
		if layer == nil {
			panic(fmt.Sprintf("layer at index %d is nil", i))
		}
		l.layers = append(l.layers, layer)
	}
	area := l.boundsWithOffset(0, 0)
	l.width = area.Dx()
	l.height = area.Dy()
	return l
}

// GetLayer returns a descendant layer by its ID, or nil if not found.
// Layers with empty IDs are skipped.
func (l *Layer) GetLayer(id string) *Layer {
	if id == "" {
		return nil
	}
	if l.id == id {
		return l
	}
	for _, child := range l.layers {
		if found := child.GetLayer(id); found != nil {
			return found
		}
	}
	return nil
}

// MaxZ returns the maximum z-index among this layer and all its descendants.
func (l *Layer) MaxZ() int {
	maxZ := l.z
	for _, child := range l.layers {
		childMaxZ := child.MaxZ()
		if childMaxZ > maxZ {
			maxZ = childMaxZ
		}
	}
	return maxZ
}

// boundsWithOffset calculates bounds with parent offset applied.
func (l *Layer) boundsWithOffset(parentX, parentY int) image.Rectangle {
	absX := l.x + parentX
	absY := l.y + parentY

	width, height := Width(l.content), Height(l.content)
	bounds := image.Rectangle{
		Min: image.Pt(absX, absY),
		Max: image.Pt(absX+width, absY+height),
	}

	for _, child := range l.layers {
		bounds = bounds.Union(child.boundsWithOffset(absX, absY))
	}

	return bounds
}

var _ uv.Drawable = (*Layer)(nil)

// Draw draws the content of the layer on the screen at the specified area.
func (l *Layer) Draw(scr uv.Screen, area uv.Rectangle) {
	content := uv.NewStyledString(l.content)
	content.Draw(scr, area)
}

// LayerHit represents the result of a hit test on a [Layer].
type LayerHit struct {
	id     string
	layer  *Layer
	bounds image.Rectangle
}

// Empty returns true if the LayerHit represents no hit.
func (lh LayerHit) Empty() bool {
	return lh.layer == nil
}

// ID returns the ID of the hit Layer.
func (lh LayerHit) ID() string {
	return lh.id
}

// Layer returns the layer that was hit.
func (lh LayerHit) Layer() *Layer {
	return lh.layer
}

// Bounds returns the bounds of the LayerHit.
func (lh LayerHit) Bounds() image.Rectangle {
	return lh.bounds
}

// Compositor manages the composition of layers. It flattens a layer hierarchy
// once and provides efficient drawing and hit testing operations. All computation
// related to layers happens in the Compositor.
type Compositor struct {
	root   *Layer
	layers []compositeLayer
	index  map[string]*Layer
	bounds image.Rectangle
}

// compositeLayer holds a flattened layer with its calculated absolute position and bounds.
type compositeLayer struct {
	layer  *Layer
	absX   int
	absY   int
	bounds image.Rectangle
}

// NewCompositor creates a new Compositor with an internal root layer. Optional
// layers can be provided which will be added as children of the root. The layer
// hierarchy is flattened and sorted by z-index for efficient rendering and hit testing.
func NewCompositor(layers ...*Layer) *Compositor {
	root := NewLayer("")
	root.AddLayers(layers...)
	c := &Compositor{
		root:  root,
		index: make(map[string]*Layer),
	}
	c.flatten()
	return c
}

// AddLayers adds layers to the compositor's root and refreshes the internal state.
func (c *Compositor) AddLayers(layers ...*Layer) *Compositor {
	c.root.AddLayers(layers...)
	c.flatten()
	return c
}

// flatten builds the internal flattened layer list and calculates overall bounds.
func (c *Compositor) flatten() {
	c.layers = nil
	c.index = make(map[string]*Layer)
	c.flattenRecursive(c.root, 0, 0)

	// Sort by absolute z-index (lowest to highest for drawing)
	slices.SortFunc(c.layers, func(a, b compositeLayer) int {
		return a.layer.z - b.layer.z
	})

	// Calculate overall bounds
	if len(c.layers) > 0 {
		c.bounds = c.layers[0].bounds
		for i := 1; i < len(c.layers); i++ {
			c.bounds = c.bounds.Union(c.layers[i].bounds)
		}
	}
}

// flattenRecursive recursively collects all layers with their absolute positions.
func (c *Compositor) flattenRecursive(layer *Layer, parentX, parentY int) {
	absX := layer.x + parentX
	absY := layer.y + parentY

	width, height := Width(layer.content), Height(layer.content)
	bounds := image.Rectangle{
		Min: image.Pt(absX, absY),
		Max: image.Pt(absX+width, absY+height),
	}

	c.layers = append(c.layers, compositeLayer{
		layer:  layer,
		absX:   absX,
		absY:   absY,
		bounds: bounds,
	})

	// Index layer by ID if it has one
	if layer.id != "" {
		c.index[layer.id] = layer
	}

	for _, child := range layer.layers {
		c.flattenRecursive(child, absX, absY)
	}
}

// Bounds returns the overall bounds of all layers in the compositor.
func (c *Compositor) Bounds() image.Rectangle {
	return c.bounds
}

// Draw draws all layers onto the given [uv.Screen] in z-index order.
func (c *Compositor) Draw(scr uv.Screen, area image.Rectangle) {
	for _, cl := range c.layers {
		if cl.bounds.Overlaps(area) {
			cl.layer.Draw(scr, cl.bounds)
		}
	}
}

// Hit performs a hit test at the given (x, y) coordinates. If a layer is hit,
// it returns the ID of the top-most layer at that point. Layers with empty IDs
// are ignored. If no layer is hit, it returns an empty [LayerHit].
func (c *Compositor) Hit(x, y int) LayerHit {
	var hit LayerHit
	pt := image.Pt(x, y)
	// Check from highest z to lowest (reverse order)
	for i := len(c.layers) - 1; i >= 0; i-- {
		cl := c.layers[i]
		if cl.layer.id != "" && pt.In(cl.bounds) {
			hit.id = cl.layer.id
			hit.layer = cl.layer
			hit.bounds = cl.bounds
			return hit
		}
	}
	return hit
}

// GetLayer returns a layer by its ID, or nil if not found.
// Layers with empty IDs are not indexed and cannot be retrieved.
func (c *Compositor) GetLayer(id string) *Layer {
	if id == "" {
		return nil
	}
	return c.index[id]
}

// Refresh re-flattens the layer hierarchy. Call this after modifying the layer
// tree structure or positions to update the compositor's internal state.
func (c *Compositor) Refresh() {
	c.flatten()
}

// Render renders the compositor into a styled string. This is a helper
// function that creates a temporary canvas, draws the compositor onto it, and
// returns the resulting string.
func (c *Compositor) Render() string {
	width, height := c.bounds.Dx(), c.bounds.Dy()
	canvas := NewCanvas(width, height)
	return canvas.Compose(c).Render()
}
