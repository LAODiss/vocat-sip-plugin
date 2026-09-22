import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Accounts from './pages/Accounts'
import CallLog from './pages/CallLog'
import Settings from './pages/Settings'
import './styles.css'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/accounts" element={<Accounts />} />
        <Route path="/call-log" element={<CallLog />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </BrowserRouter>
  )
}

createRoot(document.getElementById('root')!).render(<App />)