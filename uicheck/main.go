// Command uicheck drives the search page in headless Chrome and checks the
// parts that the Go tests do not reach: the layout at a desk width and a
// phone width, the size slider, the card view, and the filter marks. Run it
// with `mage ui:check`, which starts a local server with the fixture dataset.
//
// It is its own module, so the site's module does not depend on chromedp.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "the URL of the site")
	out := flag.String("out", "tmp/ui", "the directory for the screenshots")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o750); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	failed := false
	for _, w := range []width{{"wide", 1280, 900, false}, {"narrow", 390, 844, true}} {
		if !run(*base, *out, w) {
			failed = true
		}
	}

	if failed {
		fmt.Println("FAILED")
		os.Exit(1)
	}

	fmt.Println("ALL UI CHECKS PASS")
}

// width is one screen size.
type width struct {
	name   string
	w, h   int64
	mobile bool
}

// checker runs the checks at one width.
type checker struct {
	ctx    context.Context
	base   string
	out    string
	w      width
	failed bool
}

func run(base, out string, w width) bool {
	actx, cancel := chromedp.NewExecAllocator(
		context.Background(),
		chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	c := &checker{ctx: ctx, base: base, out: out, w: w}

	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if e.Type == "error" {
				for _, a := range e.Args {
					c.fail("console error", string(a.Value)+" "+a.Description)
				}
			}
		case *runtime.EventExceptionThrown:
			c.fail(
				"exception",
				e.ExceptionDetails.Text+" "+e.ExceptionDetails.Exception.Description,
			)
		}
	})

	c.must(chromedp.Run(ctx, emulation.SetDeviceMetricsOverride(w.w, w.h, 2, w.mobile)))

	c.pageRenders()
	c.marks()
	c.slider()
	c.sliderKeepsItsPlace()
	c.facetCountsMatch()
	c.header()
	c.flagsAndChips()
	c.activeHighlight()
	c.typeOrder()
	c.limit()
	c.redemption()
	c.cardFilters()
	c.textChanges()
	c.cardPage()

	if w.mobile {
		c.cardViewHoldsScroll()
	}

	return !c.failed
}

func (c *checker) fail(label, got string) {
	c.failed = true
	fmt.Printf("FAIL %s %s: %s\n", c.w.name, label, got)
}

func (c *checker) expect(label string, ok bool, got string) {
	if !ok {
		c.fail(label, got)

		return
	}

	fmt.Printf("PASS %s %s: %s\n", c.w.name, label, got)
}

func (c *checker) must(err error) {
	if err != nil {
		fmt.Printf("ERROR %s: %v\n", c.w.name, err)
		os.Exit(1)
	}
}

func (c *checker) wait(d time.Duration) { c.must(chromedp.Run(c.ctx, chromedp.Sleep(d))) }

func (c *checker) eval(js string) string {
	var s string
	c.must(chromedp.Run(c.ctx, chromedp.Evaluate(js, &s)))

	return s
}

func (c *checker) open(path string) {
	c.must(chromedp.Run(c.ctx, chromedp.Navigate(c.base+path), chromedp.WaitReady("body")))
	c.wait(700 * time.Millisecond)
}

func (c *checker) search(q string) { c.open("/?q=" + url.QueryEscape(q)) }

func (c *checker) shot(name string) {
	var b []byte
	c.must(
		chromedp.Run(c.ctx, chromedp.Sleep(400*time.Millisecond), chromedp.CaptureScreenshot(&b)),
	)
	c.must(os.WriteFile(filepath.Join(c.out, c.w.name+"-"+name+".png"), b, 0o600))
}

func (c *checker) click(sel string) {
	c.must(chromedp.Run(c.ctx, chromedp.Click(sel, chromedp.ByQuery)))
	c.wait(900 * time.Millisecond)
}

func (c *checker) bar() string { return c.eval(`document.getElementById('q')?.value || ''`) }

// pageRenders checks that the page shows tiles and the sidebar.
func (c *checker) pageRenders() {
	c.search("")
	n := c.eval(`String(document.querySelectorAll('.grid .tile').length)`)
	c.expect("page shows tiles", n != "0" && n != "", n+" tiles")
	f := c.eval(`String(document.querySelectorAll('.facet').length)`)
	c.expect("page shows facets", f != "0", f+" facets")
	c.shot("page")
}

// marked returns "label=state" for each marked row of a facet.
func (c *checker) marked(name string) string {
	return c.eval(
		`(() => { const sec = [...document.querySelectorAll('.facet')].find(s => s.querySelector('h2')?.innerText.trim().toLowerCase() === '` + name + `'); if (!sec) return 'none'; return [...sec.querySelectorAll('a.active, a.excluded')].map(a => a.querySelector('.label').innerText.trim() + '=' + (a.classList.contains('active') ? 'active' : 'excluded')).join('|') })()`,
	)
}

// marks checks the active filter marks of the sidebar.
func (c *checker) marks() {
	c.search("h:dis,logos")
	m := c.marked("house")
	c.expect("comma list marks both houses",
		strings.Contains(m, "Dis=active") && strings.Contains(m, "Logos=active"), m)

	c.search("-house:dis")
	m = c.marked("house")
	c.expect("negated house is struck", m == "Dis=excluded", m)

	c.click(".facet a.excluded")
	q := c.bar()
	c.expect("click removes the exclusion", !strings.Contains(q, "dis"), q)
}

// columns returns the number of grid columns.
func (c *checker) columns() string {
	return c.eval(
		`String(getComputedStyle(document.querySelector('.grid')).gridTemplateColumns.split(' ').length)`,
	)
}

func (c *checker) setSlider(v int) {
	c.eval(
		fmt.Sprintf(
			`(() => { const s = document.getElementById('size'); s.value = '%d'; s.dispatchEvent(new Event('input', {bubbles: true})); return '' })()`,
			v,
		),
	)
	c.wait(200 * time.Millisecond)
}

// slider checks the size slider: on a phone it sets 7 to 1 columns.
func (c *checker) slider() {
	c.search("")

	if c.w.mobile {
		r := c.eval(
			`(() => { const s = document.getElementById('size'); return s.min + '-' + s.max })()`,
		)
		c.expect("phone slider range", r == "1-7", r)

		for v, want := range map[int]string{1: "7", 4: "4", 7: "1"} {
			c.setSlider(v)
			got := c.columns()
			c.expect(fmt.Sprintf("phone slider %d", v), got == want, got+" columns")
		}

		c.setSlider(6)

		return
	}

	c.setSlider(120)
	small := c.columns()
	c.setSlider(300)
	large := c.columns()
	c.expect(
		"desk slider changes the columns",
		atoi(small) > atoi(large) && atoi(large) > 0,
		small+" then "+large,
	)
	c.setSlider(180)
}

// zoomOpen reports whether the card view is open.
func (c *checker) zoomOpen() bool {
	return c.eval(
		`String(!!document.getElementById('zoom') && !document.getElementById('zoom').hidden)`,
	) == "true"
}

func (c *checker) buttonState(sel string) string {
	return c.eval(
		`(() => { const b = document.querySelector('` + sel + `'); if (!b) return 'missing'; return b.classList.contains('active') ? 'active' : b.classList.contains('excluded') ? 'excluded' : 'none' })()`,
	)
}

// cardFilters checks the filter buttons of the card view.
func (c *checker) cardFilters() {
	c.search("is:reprint")
	c.click(".grid .tile")
	c.expect("card view opens", c.zoomOpen(), "")

	typeBtn := `#zoom .cv-filter[data-term^="type:"]`
	c.jsClick(typeBtn)
	q := c.bar()
	c.expect("type click adds the term", strings.Contains(q, "type:creature"), q)
	c.expect("card view stays open", c.zoomOpen(), "")
	c.expect("type value is marked", c.buttonState(typeBtn) == "active", c.buttonState(typeBtn))
	c.shot("card-filter")

	c.jsClick(typeBtn)
	q = c.bar()
	c.expect("second click removes the term", !strings.Contains(q, "type:"), q)

	// An Alt-click excludes the value.
	houseBtn := `#zoom .cv-filter[data-term^="house:"]`
	c.eval(
		`(() => { document.querySelector('` + houseBtn + `').dispatchEvent(new MouseEvent('click', {bubbles: true, altKey: true})); return '' })()`,
	)
	c.wait(900 * time.Millisecond)
	q = c.bar()
	c.expect("Alt-click excludes the house", strings.Contains(q, "-house:"), q)
	c.expect(
		"house value is struck",
		c.buttonState(houseBtn) == "excluded",
		c.buttonState(houseBtn),
	)
	c.jsClick(houseBtn)
	q = c.bar()
	c.expect("click removes the exclusion", !strings.Contains(q, "house:"), q)

	// The set chip switches the printing. The funnel next to it filters.
	shown := c.eval(`document.querySelector('#zoom .printing.active')?.dataset.print || ''`)
	c.jsClick(`#zoom .printing:not(.active)`)
	shown2 := c.eval(`document.querySelector('#zoom .printing.active')?.dataset.print || ''`)
	c.expect(
		"set chip switches the printing",
		shown2 != shown && shown2 != "",
		shown+" then "+shown2,
	)
	c.expect("set chip does not filter", !strings.Contains(c.bar(), "set:"), c.bar())
	c.jsClick(`#zoom .set-filter`)
	q = c.bar()
	c.expect("funnel adds the set", strings.Contains(q, "set:"), q)

	// An artist adds its term; the query keeps its other terms.
	a := c.eval(
		`String(!!document.querySelector('#zoom .artist:not([hidden]) .cv-filter')) + " " + [...document.querySelectorAll('#zoom .artist')].map(e => e.dataset.print + (e.hidden ? "h" : "v")).join(",") + " active=" + document.querySelector('#zoom .printing.active')?.dataset.print`,
	)
	if strings.HasPrefix(a, "true") {
		c.jsClick(`#zoom .artist:not([hidden]) .cv-filter`)
		q = c.bar()
		c.expect(
			"artist adds to the query",
			strings.Contains(q, "artist:") && strings.Contains(q, "set:"),
			q,
		)
	}

	// Closing the card view shows the new results.
	c.must(chromedp.Run(c.ctx, chromedp.KeyEvent("\x1b")))
	c.wait(600 * time.Millisecond)
	u := c.eval(`location.search`)
	c.expect("closed card view drops card=", !c.zoomOpen() && !strings.Contains(u, "card="), u)
	c.expect("URL has the new query", strings.Contains(u, "set"), u)
}

// cardViewHoldsScroll checks that a drag on the card view's backdrop does
// not scroll the page under it.
func (c *checker) cardViewHoldsScroll() {
	c.search("")
	c.setSlider(7)
	c.eval(`(() => { window.scrollTo(0, 600); return '' })()`)
	c.wait(300 * time.Millisecond)
	c.click(".grid .tile")

	before := c.eval(`String(window.scrollY)`)
	c.must(chromedp.Run(c.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.DispatchMouseEvent(input.MouseWheel, 20, float64(c.w.h-24)).
			WithDeltaY(800).
			Do(ctx)
	})))
	c.wait(600 * time.Millisecond)
	after := c.eval(`String(window.scrollY)`)
	c.expect("card view holds the page", before == after && before != "0", before+" then "+after)
	c.setSlider(6)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)

	return n
}

// cardPage checks the card page: the link preview tags that a chat app
// reads, and the card view that it opens.
func (c *checker) cardPage() {
	id := c.evalAsync(
		`(async () => { const r = await fetch('/api/cards?q=' + encodeURIComponent('n:troll')); const j = await r.json(); return j.cards[0].id })()`,
	)
	html := c.evalAsync(
		`(async () => { const r = await fetch('/card/` + id + `'); return await r.text() })()`,
	)
	tags := []string{
		`property="og:title" content="Troll"`,
		`property="og:description" content="Brobnar · Creature · P 8`,
		"Reap:",
	}
	// A printing with an image gives the preview image.
	if strings.Contains(html, `id="zoom-img"`) {
		tags = append(tags,
			`property="og:image" content="http`,
			`property="og:image:width" content="300"`,
			`property="og:image:height" content="420"`,
			`property="og:image:alt" content="Troll"`,
			`name="twitter:card" content="summary_large_image"`,
		)
	}

	for _, tag := range tags {
		c.expect("card page has "+tag, strings.Contains(html, tag), id)
	}

	// The preview has no flavor text. The card view on the page has it.
	desc := html
	if _, after, ok := strings.Cut(html, `property="og:description" content="`); ok {
		desc, _, _ = strings.Cut(after, `"`)
	}

	c.expect("the preview has no flavor text", !strings.Contains(desc, "feed it"), desc)

	c.open("/card/" + id)
	u := c.eval(`location.pathname + location.search`)
	c.expect("card page opens the card view", c.zoomOpen() && u == "/?card="+id, u)
	c.must(chromedp.Run(c.ctx, chromedp.KeyEvent("\x1b")))
	c.wait(500 * time.Millisecond)
	u = c.eval(`location.pathname + location.search`)
	c.expect("closing the card page shows the search", !c.zoomOpen() && u == "/", u)
}

// evalAsync runs a script that returns a promise and returns its value.
func (c *checker) evalAsync(js string) string {
	var s string
	c.must(
		chromedp.Run(
			c.ctx,
			chromedp.Evaluate(js, &s, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
				return p.WithAwaitPromise(true)
			}),
		),
	)

	return s
}

// jsClick clicks the first element that matches the selector with a script,
// so a missing element fails the check at once instead of waiting.
func (c *checker) jsClick(sel string) {
	ok := c.eval(
		`(() => { const e = document.querySelector('` + sel + `'); if (!e) return 'missing'; e.click(); return 'ok' })()`,
	)
	if ok != "ok" {
		c.fail("click "+sel, ok)

		return
	}

	c.wait(900 * time.Millisecond)
}

// textChanges checks the text of each printing: is:changed opens on a
// printing with the change icon, a set chip switches the card text, and
// "All sets" shows every printing with the changed words marked.
func (c *checker) textChanges() {
	c.search("is:changed")
	c.click(".grid .tile")
	c.expect("is:changed opens a card", c.zoomOpen(), "")

	off := c.eval(
		`(() => { const b = document.querySelector("#zoom .set-filter"); const p = b?.querySelector("path"); if (!p) return "none"; const r = b.getBoundingClientRect(), q = p.getBoundingClientRect(); return Math.max(Math.abs((r.left + r.right - q.left - q.right) / 2), Math.abs((r.top + r.bottom - q.top - q.bottom) / 2)).toFixed(2) })()`,
	)
	d, err := strconv.ParseFloat(off, 64)
	c.expect("the set funnel is centered within 1px", err == nil && d <= 1, off)

	icon := c.eval(`String(!!document.querySelector('#zoom .printing.active .text-changed'))`)
	c.expect("the shown printing has the change icon", icon == "true", icon)

	shown := `[...document.querySelectorAll('#zoom .zoom-text')].filter(e => !e.hidden)`
	n := c.eval(`String(` + shown + `.length)`)
	c.expect("one card text shows", n == "1", n)

	before := c.eval(shown + `[0]?.dataset.print || ''`)
	c.jsClick(`#zoom .printing:not(.active)`)
	after := c.eval(shown + `[0]?.dataset.print || ''`)
	c.expect("a set chip switches the card text", after != "" && after != before, before+" "+after)

	c.jsClick(`#zoom .all-sets-toggle`)
	open := c.eval(`String(!document.getElementById('all-sets')?.hidden)`)
	c.expect("All sets opens", open == "true", open)

	figs := c.eval(`String(document.querySelectorAll('#all-sets figure').length)`)
	c.expect("All sets shows every printing", atoi(figs) >= 2, figs)

	marks := c.eval(`String(document.querySelectorAll('#all-sets mark.diff').length)`)
	c.expect("All sets marks the changed words", atoi(marks) >= 1, marks)
	c.shot("all-sets")

	c.jsClick(`#zoom .zoom-close`)
}

// typeQuery types a query into the search bar, as a user does, so the
// results change without a page load.
func (c *checker) typeQuery(q string) {
	c.eval(fmt.Sprintf(
		`(() => { const i = document.getElementById('q'); i.value = %q; i.dispatchEvent(new Event('input', {bubbles: true})); return '' })()`,
		q,
	))
	c.wait(1800 * time.Millisecond)
}

// sliderState returns the slider value and its filled part.
func (c *checker) sliderState() string {
	return c.eval(
		`(() => { const s = document.getElementById('size'); return s.min + '-' + s.max + ':' + s.value + ':' + s.style.getPropertyValue('--fill') })()`,
	)
}

// sliderKeepsItsPlace checks that a search does not move the slider's thumb.
func (c *checker) sliderKeepsItsPlace() {
	c.search("")

	v := 260
	if c.w.mobile {
		v = 5
	}

	c.setSlider(v)
	before := c.sliderState()
	c.typeQuery("h:dis")
	after := c.sliderState()
	c.expect("slider keeps its place after a search", before == after, before+" then "+after)
}

// count returns the total of the results: the number before limit:N, so
// "10 of 2,457" is 2457.
func (c *checker) count() int {
	s := c.eval(`document.querySelector('.count')?.innerText || ''`)
	f := strings.Fields(strings.ReplaceAll(s, ",", ""))
	n := 0

	for _, w := range f {
		if v := atoi(w); v > 0 {
			n = v
		}
	}

	return n
}

// facetRow returns the label and count of the first row of a facet that the
// query does not filter on, or "".
func (c *checker) facetRow(name string) (string, int) {
	s := c.eval(
		`(() => { const sec = [...document.querySelectorAll('.facet')].find(s => s.querySelector('h2')?.innerText.trim().toLowerCase() === '` + name + `'); if (!sec) return ''; const a = sec.querySelector('a:not(.active):not(.excluded)'); if (!a) return ''; a.setAttribute('data-check', '1'); return a.querySelector('.label').innerText.trim() + '|' + a.querySelector('.n').innerText.trim() })()`,
	)

	label, n, ok := strings.Cut(s, "|")
	if !ok {
		return "", 0
	}

	return label, atoi(strings.ReplaceAll(n, ",", ""))
}

// facetCountsMatch checks that a facet count is the number of results that
// a click on the value finds.
func (c *checker) facetCountsMatch() {
	facets := []string{
		"house", "type", "rarity", "set", "keyword", "ability", "trait", "artist", "flags",
		"power",
	}

	// A query whose value the data does not have gives an error page with
	// no facets, so its rows are skipped.
	for _, q := range []string{
		"", "type:reference", "h:redemption", "s:wc", "is:anomaly", "r:anomaly",
		"is:eviltwin", "k:assault", "-h:dis", "-h:brobnar,logos t:creature",
		"sort:random limit:10",
	} {
		for _, name := range facets {
			c.search(q)

			label, n := c.facetRow(name)
			if label == "" {
				continue
			}

			c.jsClick(`.facet a[data-check="1"]`)
			c.wait(1200 * time.Millisecond)
			got := c.count()
			c.expect(
				fmt.Sprintf("%q: %s %s count matches its click", q, name, label),
				got == n,
				fmt.Sprintf("facet %d, results %d", n, got),
			)
		}
	}
}

// header checks the header buttons: a menu on a phone, and a Random icon
// on a narrow header.
func (c *checker) header() {
	c.search("")

	if c.w.mobile {
		hidden := c.eval(`getComputedStyle(document.getElementById('menu-items')).display`)
		c.expect("phone menu starts closed", hidden == "none", hidden)
		c.click("#menu")
		shown := c.eval(`getComputedStyle(document.getElementById('menu-items')).display`)
		label := c.eval(`getComputedStyle(document.querySelector('#random .label')).display`)
		c.expect("menu button opens the menu", shown == "flex" && label != "none", shown+" "+label)
		c.shot("menu")
		c.click("#menu")

		return
	}

	menu := c.eval(`getComputedStyle(document.getElementById('menu')).display`)
	c.expect("desk has no menu button", menu == "none", menu)

	c.must(chromedp.Run(c.ctx, emulation.SetDeviceMetricsOverride(900, c.w.h, 1, false)))
	c.wait(300 * time.Millisecond)
	icon := c.eval(`getComputedStyle(document.querySelector('#random .random-icon')).display`) +
		"/" + c.eval(`getComputedStyle(document.querySelector('#random .label')).display`)
	c.expect("narrow header shows the Random icon", icon == "block/none", icon)
	c.must(chromedp.Run(c.ctx, emulation.SetDeviceMetricsOverride(c.w.w, c.w.h, 2, false)))
	c.wait(300 * time.Millisecond)
}

// flagsAndChips checks the Flags facet and the set codes of the card view.
func (c *checker) flagsAndChips() {
	c.search("")

	flags := c.eval(
		`(() => { const sec = [...document.querySelectorAll('.facet')].find(s => s.querySelector('h2')?.innerText.trim().toLowerCase() === 'flags'); return sec ? [...sec.querySelectorAll('.label')].map(l => l.innerText.trim()).join('|') : '' })()`,
	)
	c.expect(
		"flags facet shows flag names",
		flags != "" && !strings.Contains(flags, "tokencreature"),
		flags,
	)

	c.search("is:reprint")
	c.click(".grid .tile")
	chip := c.eval(`document.querySelector('#zoom .printing')?.innerText.trim() || ''`)
	name := c.eval(`document.querySelector('#zoom .printing')?.title || ''`)
	c.expect("set chip shows the set code", chip != "" && len(chip) < len(name), chip+" / "+name)
}

// activeRow returns how the marked row of a facet looks: its label, the
// remove mark, and its background, or "none".
func (c *checker) activeRow(name string) string {
	return c.eval(
		`(() => { const sec = [...document.querySelectorAll('.facet')].find(s => s.querySelector('h2')?.innerText.trim().toLowerCase() === '` + name + `'); const a = sec?.querySelector('a.active'); if (!a) return 'none'; const bg = getComputedStyle(a).backgroundColor; return a.querySelector('.label').innerText.trim() + '|' + (a.querySelector('.x') ? 'x' : 'no-x') + '|' + (bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent' ? 'filled' : 'clear') })()`,
	)
}

// activeHighlight checks that an active filter is clearly marked in the
// sidebar, also after the results change by typing and by a click.
func (c *checker) activeHighlight() {
	c.search("h:dis")
	got := c.activeRow("house")
	// A marked row has no remove mark: the user wants the count in place.
	c.expect("active house is highlighted", got == "Dis|no-x|filled", got)

	c.typeQuery("h:dis t:creature")
	got = c.activeRow("house") + " " + c.activeRow("type")
	c.expect("highlight after typing", got == "Dis|no-x|filled Creature|no-x|filled", got)

	c.search("")
	label, _ := c.facetRow("flags")
	c.jsClick(`.facet a[data-check="1"]`)
	c.wait(1200 * time.Millisecond)
	got = c.activeRow("flags")
	c.expect("highlight after a click", strings.HasPrefix(got, label+"|no-x|filled"), got)
	c.shot("active")
}

// typeOrder checks that Reference is the last type.
func (c *checker) typeOrder() {
	c.search("")
	types := c.eval(
		`(() => { const sec = [...document.querySelectorAll('.facet')].find(s => s.querySelector('h2')?.innerText.trim().toLowerCase() === 'type'); return sec ? [...sec.querySelectorAll('.label')].map(l => l.innerText.trim()).join('|') : '' })()`,
	)
	c.expect("reference is the last type",
		!strings.Contains(types, "Reference") || strings.HasSuffix(types, "Reference"), types)
}

// limit checks limit:N: N tiles, and the count shows N of the total.
func (c *checker) limit() {
	c.search("sort:random limit:10")
	count := c.eval(`document.querySelector('.count')?.innerText.replace(/\s+/g, ' ').trim() || ''`)
	tiles := c.eval(`String(document.querySelectorAll('.grid .tile').length)`)
	c.expect("limit:10 shows 10 of the total",
		strings.HasPrefix(count, "10 of ") && tiles == "10", count+", "+tiles+" tiles")
}

// redemption checks a Redemption printing: h:redemption shows it, and its
// set chip has the Redemption icon. The fixture has no Redemption printing.
func (c *checker) redemption() {
	c.search("h:redemption is:reprint")
	if c.eval(`String(document.querySelectorAll('.grid .tile').length)`) == "0" {
		fmt.Printf("SKIP %s redemption: no Redemption printings in the data\n", c.w.name)

		return
	}

	c.click(".grid .tile")
	icons := c.eval(
		`[...document.querySelectorAll('#zoom .printing')].map(b => b.querySelectorAll('.ico').length + ':' + b.title).join(' | ')`,
	)
	c.expect("a Redemption chip has the house icon", strings.Contains(icons, "Redemption"), icons)
	c.shot("redemption")
}
