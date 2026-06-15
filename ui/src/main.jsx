import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
// Carbon precompiled CSS — avoids Vite/Sass package-export resolution issues
import '@carbon/styles/css/styles.css'
import './styles/index.css'

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
