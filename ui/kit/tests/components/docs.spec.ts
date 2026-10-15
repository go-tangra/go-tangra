import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { defineComponent, h } from 'vue'
import { expectA11y, mountBody } from '../helpers'
import { UiDocContent, UiHelpLink } from '@/index'

// Shaped like the gateway's rendered help HTML (goldmark + chroma classes + bluemonday).
const HTML = `<h2 id="how-it-works">How it works</h2>
<p>See <a href="/help/deployer/guides/targets#filters">targets</a>, <a href="#example">below</a>,
<a href="https://example.org/" rel="nofollow noreferrer noopener" target="_blank">elsewhere</a> or <a href="/ops">ops</a>.</p>
<h2 id="example">Example</h2>
<pre class="chroma"><code><span class="line"><span class="cl"><span class="nt">provider</span><span class="p">:</span> bigip
</span></span></code></pre>
<pre><code>plain
</code></pre>
<p><mark>hit</mark> <img src="/gateway/v1/docs/deployer/assets/a.png?v=ab" alt="diagram"></p>`

describe('UiDocContent', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('renders the HTML in a prose container and gives every code block a copy button', async () => {
    const w = mountBody(UiDocContent, { props: { html: HTML, label: 'Help page' } })
    await flushPromises()
    const root = w.find('article.ui-doc-content')
    expect(root.attributes('aria-label')).toBe('Help page')
    expect(root.find('h2#how-it-works').text()).toBe('How it works')
    expect(root.find('.chroma .nt').text()).toBe('provider')
    const blocks = root.findAll('.ui-doc-code')
    expect(blocks).toHaveLength(2)
    for (const b of blocks) {
      expect(b.find('pre').attributes('tabindex')).toBe('0')
      expect(b.find('button.ui-doc-copy').attributes('aria-label')).toBe('Copy code')
      expect(b.find('button.ui-doc-copy span').classes()).toContain('icon-[mdi--content-copy]')
    }
    // no inline handlers or styles are introduced
    expect(root.element.innerHTML).not.toMatch(/\son[a-z]+=|style=/)
    await expectA11y(w.element)
    w.unmount()
  })

  it('copies the code block text and confirms', async () => {
    vi.useFakeTimers()
    const writeText = vi.fn(async () => undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const w = mountBody(UiDocContent, { props: { html: HTML } })
    await flushPromises()
    const btn = w.find('button.ui-doc-copy')
    await btn.trigger('click')
    await flushPromises()
    expect(writeText).toHaveBeenCalledWith('provider: bigip\n')
    expect(btn.attributes('aria-label')).toBe('Copied')
    expect(btn.find('span').classes()).toContain('icon-[mdi--check]')
    vi.advanceTimersByTime(1600)
    expect(btn.attributes('aria-label')).toBe('Copy code')
    vi.useRealTimers()
    w.unmount()
  })

  it('keeps quiet when the clipboard is unavailable', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn(async () => { throw new Error('denied') }) } })
    const w = mountBody(UiDocContent, { props: { html: HTML } })
    await flushPromises()
    await w.find('button.ui-doc-copy').trigger('click')
    await flushPromises()
    expect(w.find('button.ui-doc-copy').attributes('aria-label')).toBe('Copy code')
    w.unmount()
  })

  it('emits navigate for help links and in-page anchors, leaves other links alone', async () => {
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    const w = mountBody(UiDocContent, { props: { html: HTML } })
    await flushPromises()
    const click = (sel: string, init: MouseEventInit = {}) => {
      const ev = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init })
      w.find(sel).element.dispatchEvent(ev)
      return ev.defaultPrevented
    }
    expect(click('a[href^="/help/"]')).toBe(true)
    expect(w.emitted('navigate')?.[0]).toEqual(['/help/deployer/guides/targets#filters'])
    expect(click('a[href="#example"]')).toBe(true)
    expect(w.emitted('navigate')?.[1]).toEqual(['#example'])
    expect(scroll).toHaveBeenCalledTimes(1)
    expect(click('a[target="_blank"]')).toBe(false)
    expect(click('a[href="/ops"]')).toBe(false)
    expect(click('a[href^="/help/"]', { ctrlKey: true })).toBe(false)
    expect(w.emitted('navigate')).toHaveLength(2)
    w.unmount()
  })

  it('scrolls to the hash target and re-enhances new content', async () => {
    const scrolled: string[] = []
    Element.prototype.scrollIntoView = function (this: Element) { scrolled.push(this.id) }
    const w = mountBody(UiDocContent, { props: { html: HTML, hash: '#example' } })
    await flushPromises()
    expect(scrolled).toEqual(['example'])
    await w.setProps({ hash: 'how-it-works' })
    await flushPromises()
    expect(scrolled).toEqual(['example', 'how-it-works'])
    await w.setProps({ html: '<h2 id="other">Other</h2><pre><code>x</code></pre>', hash: undefined })
    await flushPromises()
    expect(w.findAll('.ui-doc-copy')).toHaveLength(1)
    ;(w.vm as unknown as { scrollTo: (h: string) => void }).scrollTo('#missing')
    expect(scrolled).toHaveLength(2)
    w.unmount()
  })
})

describe('UiHelpLink', () => {
  it('links to the help page with an encoded slug and anchor', async () => {
    const w = mount(UiHelpLink, { props: { module: 'deployer', slug: 'guides/auto deploy', anchor: '#set-up' } })
    expect(w.attributes('href')).toBe('/help/deployer/guides/auto%20deploy#set-up')
    expect(w.text()).toBe('Help')
    expect(w.find('.icon-\\[mdi--help-circle-outline\\]').exists()).toBe(true)
    // no router: a plain link the browser follows
    const ev = new MouseEvent('click', { bubbles: true, cancelable: true })
    w.element.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(false)
  })

  it('is icon-only with an accessible name and navigates through the host router', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:p(.*)*', component: defineComponent({ render: () => h('div') }) }] })
    await router.push('/')
    const w = mountBody(UiHelpLink, { props: { module: 'ipam', slug: 'subnets', iconOnly: true, label: 'Help: subnets', size: 'md' }, global: { plugins: [router] } })
    expect(w.attributes('aria-label')).toBe('Help: subnets')
    expect(w.text()).toBe('')
    expect(w.classes()).toContain('btn-circle')
    await w.trigger('click', { button: 0 })
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/help/ipam/subnets')
    await w.trigger('click', { button: 0, metaKey: true })
    await expectA11y(w.element)
    w.unmount()
  })
})
