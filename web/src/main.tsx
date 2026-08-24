import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { ThemeProvider } from './context/ThemeContext'
import './index.css'

// #region agent log
try {
  const po = new PerformanceObserver(list => {
    for (const e of list.getEntries()) {
      fetch('http://127.0.0.1:7473/ingest/49c06dfb-d2a4-42ad-83ca-3f7de467dc84',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'f7d7ab'},body:JSON.stringify({sessionId:'f7d7ab',runId:'pre-fix',hypothesisId:'C',location:'main.tsx:lcp',message:'LCP entry',data:{name:e.name,startTime:Math.round(e.startTime),size:(e as PerformanceEntry & {size?:number}).size,url:location.pathname},timestamp:Date.now()})}).catch(()=>{})
    }
  })
  po.observe({ type: 'largest-contentful-paint', buffered: true } as PerformanceObserverInit)
  void document.fonts.ready.then(() => {
    const fonts = performance.getEntriesByType('resource').filter(r => /fonts\.(googleapis|gstatic)/.test(r.name)).map(r => ({ name: r.name.split('?')[0], dur: Math.round(r.duration), transfer: Math.round((r as PerformanceResourceTiming).transferSize || 0) }))
    fetch('http://127.0.0.1:7473/ingest/49c06dfb-d2a4-42ad-83ca-3f7de467dc84',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'f7d7ab'},body:JSON.stringify({sessionId:'f7d7ab',runId:'pre-fix',hypothesisId:'C',location:'main.tsx:fonts',message:'Font resources',data:{readyAt:Math.round(performance.now()),fonts},timestamp:Date.now()})}).catch(()=>{})
  })
} catch { /* ignore */ }
// #endregion

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ThemeProvider>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ThemeProvider>
  </React.StrictMode>,
)
