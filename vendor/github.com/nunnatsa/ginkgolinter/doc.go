package ginkgolinter

const doc = `enforces standards of using ginkgo and gomega

or
       ginkgolinter version

version: %s

currently, the linter searches for following:
* trigger a warning when using Eventually or Consistently with a function call. This is in order to prevent the case when 
  using a function call instead of a function. Function call returns a value only once, and so the original value
  is tested again and again and is never changed. [Bug]

* trigger a warning when comparing a pointer to a value. [Bug]

* trigger a warning for missing assertion method: [Bug]
	Eventually(checkSomething)

* trigger a warning when a ginkgo focus container (FDescribe, FContext, FWhen or FIt) is found. [Bug]

* validate the MatchError gomega matcher [Bug]

* trigger a warning when using the Equal or the BeIdentical matcher with two different types, as these matchers will
  fail in runtime.

* async timing interval: timeout is shorter than polling interval [Bug]
For example:
	Eventually(aFunc).WithTimeout(500 * time.Millisecond).WithPolling(10 * time.Second).Should(Succeed())
This will probably happen when using the old format:
	Eventually(aFunc, 500 * time.Millisecond, 10 * time.Second).Should(Succeed())

* Success matcher validation: [BUG]
  The Success matcher expect that the actual argument will be a single error. In async actual assertions, It also allow 
  functions with Gomega object as the function first parameter.
For example:
  Expect(myInt).To(Succeed())
or
  Eventually(func() int { return 42 }).Should(Succeed())

* reject variable assignments in ginkgo containers [Bug/Style]:
For example:
	var _ = Describe("description", func(){
		var x = 10
	})

Should use BeforeEach instead; e.g.
	var _ = Describe("description", func(){
		var x int
		BeforeEach(func(){
			x = 10
		})
	})

* wrong length assertions. We want to assert the item rather than its length. [Style]
For example:
	Expect(len(x)).Should(Equal(1))
This should be replaced with:
	Expect(x)).Should(HavelLen(1))
	
* wrong cap assertions. We want to assert the item rather than its cap. [Style]
For example:
	Expect(cap(x)).Should(Equal(1))
This should be replaced with:
	Expect(x)).Should(HavelCap(1))
	
* wrong nil assertions. We want to assert the item rather than a comparison result. [Style]
For example:
	Expect(x == nil).Should(BeTrue())
This should be replaced with:
	Expect(x).Should(BeNil())

* wrong error assertions. For example: [Style]
	Expect(err == nil).Should(BeTrue())
This should be replaced with:
	Expect(err).ShouldNot(HaveOccurred())

* wrong boolean comparison, for example: [Style]
	Expect(x == 8).Should(BeTrue())
This should be replaced with:
	Expect(x).Should(BeEqual(8))

* replaces Equal(true/false) with BeTrue()/BeFalse() [Style]

* replaces HaveLen(0) with BeEmpty() [Style]

* replaces Expect(...).Should(...) with Expect(...).To() [Style]

* async timing interval: multiple timeout or polling interval [Style]
For example:
	Eventually(context.Background(), func() bool { return true }, time.Second*10).WithTimeout(time.Second * 10).WithPolling(time.Millisecond * 500).Should(BeTrue())
	Eventually(context.Background(), func() bool { return true }, time.Second*10).Within(time.Second * 10).WithPolling(time.Millisecond * 500).Should(BeTrue())
	Eventually(func() bool { return true }, time.Second*10, 500*time.Millisecond).WithPolling(time.Millisecond * 500).Should(BeTrue())
	Eventually(func() bool { return true }, time.Second*10, 500*time.Millisecond).ProbeEvery(time.Millisecond * 500).Should(BeTrue())

* async timing interval: non-time.Duration intervals [Style]
gomega supports a few formats for timeout and polling intervals, when using the old format (the last two parameters of Eventually and Consistently):
  * time.Duration
  * any kind of numeric value, as number of seconds
  * duration string like "12s"
The linter triggers a warning for any duration value that is not of the time.Duration type, assuming that this is
the desired type, given the type of the argument of the newer "WithTimeout", "WithPolling", "Within" and "ProbeEvery" 
methods. 
For example:
	Eventually(context.Background(), func() bool { return true }, "1s").Should(BeTrue())
	Eventually(context.Background(), func() bool { return true }, time.Second*60, 15).Should(BeTrue())

* Success <=> Eventually usage [Style]
  enforces that the Succeed() matcher will be used for error functions, and the HaveOccurred() matcher will
  be used for error values.

For example:
  Expect(err).ToNot(Succeed())
or
  Expect(funcRetError().ToNot(HaveOccurred())

* force assertion descriptions [Style]
  enforces that all assertions include an optional description message to improve test readability and debugging.
  This rule is disabled by default. Use the --force-assertion-description flag to enable it.

For example:
  Expect("hello").To(Equal("hello")) // This will trigger a warning
Should be:
  Expect("hello").To(Equal("hello"), "greeting should match")

The rule also works with async assertions and Expect calls inside Eventually:
  Eventually(func() {
    Expect(value).To(Equal(expected)) // This will also trigger a warning if no description
  }).Should(Succeed(), "operation should complete successfully")

* Force ToNot [Style]
  enforces that ToNot() or ShouldNot() are used instead of To(Not()) or Should(Not()).
`
