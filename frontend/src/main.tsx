import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { ToastContainer } from 'react-toastify'
import 'react-toastify/dist/ReactToastify.css'
import App from './App'
import { PortfolioProvider } from './context/PortfolioContext'
import './styles.css'

const container = document.getElementById('app')
if (!container) throw new Error('Root element #app not found')

createRoot(container).render(
  <BrowserRouter>
    <PortfolioProvider>
      <App />
    </PortfolioProvider>
    <ToastContainer position="top-right" newestOnTop limit={20} />
  </BrowserRouter>,
)
