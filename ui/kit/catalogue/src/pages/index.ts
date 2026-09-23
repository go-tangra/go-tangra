import type { Component } from 'vue'
import Primitives from './Primitives.vue'
import Overlays from './Overlays.vue'
import Fields from './Fields.vue'
import Data from './Data.vue'
import Forms from './Forms.vue'
import Composite from './Composite.vue'
import Shell from './Shell.vue'

export interface CataloguePage { key: string; title: string; component: Component }
export const pages: CataloguePage[] = [
  { key: 'primitives', title: 'Primitives', component: Primitives },
  { key: 'overlays', title: 'Overlays', component: Overlays },
  { key: 'fields', title: 'Fields', component: Fields },
  { key: 'data', title: 'Data', component: Data },
  { key: 'forms', title: 'Forms', component: Forms },
  { key: 'composite', title: 'Composite', component: Composite },
  { key: 'shell', title: 'App shell', component: Shell },
]
