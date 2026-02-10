import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

import './styles/reset.css'
import './styles/variables.css'
import './styles/layout.css'

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
