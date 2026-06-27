# mtex

`mtex` provides a Go implementation of a naive LaTeX-like math expression parser and renderer.

## Example

```
$> mtex-render -font-size=48 -dpi=100 "$\sum\sqrt{\frac{a+b}{2\pi}}\cos\omega\binom{a+b}{\beta}\prod \alpha x\int\frac{\partial x}{x}\hbar$"
```

![mtex-example](https://codeberg.org/go-latex/latex/raw/master/mtex/testdata/mtex-example.png)
