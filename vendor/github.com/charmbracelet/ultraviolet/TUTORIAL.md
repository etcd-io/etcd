# Hello World!

This is a simple tutorial on how to create a UV application that displays
"Hello, World!" on the screen. UV is a terminal UI toolkit that allows you to
create terminal applications with ease. It provides a simple API to handle
screen management, input handling, and rendering content.

## Tutorial

What does UV consist of? A UV application consists of a screen that displays
content, has some sort of input sources that can be used to interact with the
screen, and is meant to display content or programs on the screen.

First, we need to create a screen that will display our program. UV comes with
a `Terminal` screen that is used to display content on a terminal.

```go
t := uv.NewTerminal(os.Stdin, os.Stdout, os.Environ())
// Or simply use...
// t := uv.DefaultTerminal()
```

A terminal screen has a few properties that are unique to it. For example, a
terminal screen can go into raw mode, which is important to disable echoing of
input characters, and to disable signal handling so that we can receive things
like <kbd>ctrl+c</kbd> without the terminal interfering with our program.

Another important property of a terminal screen is the alternate screen buffer.
This property puts the terminal screen into a special mode that allows us to
display content without interfering with the normal screen buffer.

In this tutorial, we will use the alternate screen buffer to display our
program so that we don't affect the normal screen buffer.

```go
// Set the terminal to raw mode.
if err := t.MakeRaw(); err != nil {
  log.Fatal(err)
}

// Enter the alternate screen buffer. This will
// only take affect once we flush or display
// our program on the terminal screen.
t.EnterAltScreen()

// My program
// ...

// Make sure we leave the alternate screen buffer when we
// are done with our program. This will be called automatically
// when we use [t.Shutdown(ctx)] later.
t.LeaveAltScreen()

// Make sure we restore the terminal to its original state
// before we exit. We don't care about errors here, but you
// can handle them if you want. This will be called automatically
// when we use [t.Shutdown(ctx)] later.
_ = t.Restore() //nolint:errcheck
```

Now that we have our screen set to raw mode and in the alternate screen buffer,
we can create our program that will be displayed on the screen. A program is an
abstraction layer that handles different screen types and implementations. It
only cares about displaying content on the screen.

We need to start our program before we can display anything on the screen. This
will ensure that the program and screen are initialized and ready to display
content. Internally, this will also call `t.Start()` to start the terminal
screen.

```go
if err := t.Start(); err != nil {
  log.Fatalf("failed to start program: %v", err)
}
```

Let's display a simple frame with some text in it. A frame is a container that
holds the buffer we're displaying. The final cursor position we want our cursor
to be at, and the viewport area we are working with to display our content.

```go
for i, r := range "Hello, World!" {
  // We iterate over the string to display each character
  // in a separate cell. Ideally, we want each cell
  // to have exactly one grapheme. In this case, since
  // we're using a simple ASCII string, we know that
  // each character is a single grapheme with a width of 1.
  var c uv.Cell
  c.Content = string(r)
  c.Width = 1
  t.SetCell(i, 0, &c)
}
// Now we simply render the changes and flush them
// to the terminal screen.
_ = p.Display()
```

Different screen models have different ways to receive input. Some models have
a remote control, while others have a touch screen. A terminal can receive
input from various peripherals usually through control codes and escape
sequences. Our terminal has a `t.Events(ctx)` method that returns a channel
which will receive events from different terminal input sources.

```go
// We want to be able to stop the terminal input loop
// whenever we call cancel().
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for ev := range t.Events(ctx) {
  switch ev := ev.(type) {
  case uv.WindowSizeEvent:
    // Our terminal screen is resizable. This is important
    // as we want to inform our terminal screen with the
    // size we'd like to display our program in.
    // When we're using the full terminal window size,
    // we can assume that the terminal screen will
    // also have the same size as our program.
    // However, with inline programs, usually we want
    // the height to be the height of our program.
    // So if our inline program takes 10 lines, we
    // want to resize the terminal screen to 10 lines
    // high.
    width, height := ev.Width, ev.Height
    if !altscreen {
      height = 10
    }
    t.Resize(width, height)
  case uv.KeyPressEvent:
    if ev.MatchStrings("q", "ctrl+c") {
      // This will stop the input loop and cancel the context.
      cancel()
    }
  }
}
```

Now that we've handled displaying our program and receiving input from the
terminal, we need to handle the program's lifecycle. We need to make sure that
we restore the terminal to its original state when we exit the program. A
terminal program can be stopped gracefully using the `t.Shutdown(ctx)` method.

```go
// We need to make sure we stop the program gracefully
// after we exit the input loop.
if err := t.Shutdown(ctx); err != nil {
  log.Fatal(err)
}
```

Finally, let's put everything together and create a simple program that displays
a frame with "Hello, World!" in it. The program will exit when we press
<kbd>ctrl+c</kbd> or <kbd>q</kbd>.

```go
func main() {
	t := uv.NewTerminal(os.Stdin, os.Stdout, os.Environ())

	if err := t.MakeRaw(); err != nil {
		log.Fatalf("failed to make terminal raw: %v", err)
	}

	if err := t.Start(); err != nil {
		log.Fatalf("failed to start program: %v", err)
	}

	t.EnterAltScreen()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for ev := range t.Events(ctx) {
		switch ev := ev.(type) {
		case uv.WindowSizeEvent:
			width, height := ev.Width, ev.Height
			t.Erase()
			t.Resize(width, height)
		case uv.KeyPressEvent:
			if ev.MatchStrings("q", "ctrl+c") {
				cancel()
			}
		}

		for i, r := range "Hello, World!" {
			var c uv.Cell
			c.Content = string(r)
			c.Width = 1
			t.SetCell(i, 0, &c)
		}
		if err := t.Display(); err != nil {
			log.Fatal(err)
		}
	}

	if err := t.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
```

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400"></a>

Charm热爱开源 • Charm loves open source • نحنُ نحب المصادر المفتوحة
