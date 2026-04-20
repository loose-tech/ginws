import './style.css'
import { Ws } from '../../../client/ws'

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <div style="padding: 20px; text-align: left;">
    <h1>GinWS Demo</h1>
    <div id="status" style="margin-bottom: 10px;">Status: Connecting...</div>
    
    <div style="margin-bottom: 20px;">
      <button id="sendHello">Send Hello (with reply)</button>
      <button id="sendEcho">Send Echo</button>
      <button id="sendSlowEcho">Send Slow Echo (timeout test)</button>
    </div>

    <div style="margin-bottom: 10px;">
      <input type="text" id="chatInput" placeholder="Type a message..." />
      <button id="sendChat">Send Chat</button>
    </div>

    <div id="logs" style="background: #1e1e1e; color: #d4d4d4; padding: 10px; height: 400px; overflow-y: auto; font-family: monospace; border-radius: 4px;">
    </div>
  </div>
`

const logContainer = document.querySelector<HTMLDivElement>('#logs')!
const statusContainer = document.querySelector<HTMLDivElement>('#status')!

function log(msg: string, type: 'info' | 'success' | 'error' | 'warn' = 'info') {
  const entry = document.createElement('div')
  const colors = {
    info: '#d4d4d4',
    success: '#4ec9b0',
    error: '#f44747',
    warn: '#dcdcaa'
  }
  entry.style.color = colors[type]
  entry.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`
  logContainer.appendChild(entry)
  logContainer.scrollTop = logContainer.scrollHeight
}

// Use relative URL to leverage Vite proxy
const ws = new Ws(`ws://localhost:5000/ws`)

ws.on('open', () => {
  statusContainer.textContent = 'Status: Connected'
  statusContainer.style.color = '#4ec9b0'
  log('WebSocket connected', 'success')
})

ws.on('close', () => {
  statusContainer.textContent = 'Status: Disconnected'
  statusContainer.style.color = '#f44747'
  log('WebSocket disconnected', 'error')
})

ws.on('error', (err: any) => {
  log(`WebSocket error: ${err}`, 'error')
})

// Listen for broadcasted chat messages
ws.use('chat_message', (msg) => {
  log(`[Chat] ${msg.payload}`, 'info')
  return ""
})

document.querySelector('#sendHello')?.addEventListener('click', async () => {
  log('Sending "hello" request...')
  try {
    await ws.sendRaw('hello', null, null, 2000, (reply) => {
      log(`Received reply for hello: ${reply.payload}`, 'success')
      return ""
    }, () => {
      log('Hello request expired', 'warn')
    })
  } catch (e) {
    log(`Error sending hello: ${e}`, 'error')
  }
})

document.querySelector('#sendEcho')?.addEventListener('click', async () => {
  const payload = "Hello Echo " + Math.random().toString(36).substring(7)
  log(`Sending "echo" with payload: ${payload}`)
  try {
    await ws.sendRaw('echo', null, payload, 2000, (reply) => {
      log(`Received echo response: ${reply.payload}`, 'success')
      return ""
    })
  } catch (e) {
    log(`Error sending echo: ${e}`, 'error')
  }
})

document.querySelector('#sendSlowEcho')?.addEventListener('click', async () => {
  log('Sending "slow_echo" (expecting timeout in 1s, server takes 2s)...')
  try {
    // 1000ms duration, but server takes 2000ms
    await ws.sendRaw('slow_echo', null, "slow", 1000, (reply) => {
      log(`Received slow_echo response: ${reply.payload}`, 'success')
      return ""
    }, () => {
      log('Slow echo timed out as expected!', 'warn')
    })
  } catch (e) {
    log(`Error sending slow_echo: ${e}`, 'error')
  }
})

document.querySelector('#sendChat')?.addEventListener('click', () => {
  const input = document.querySelector<HTMLInputElement>('#chatInput')!
  const val = input.value
  if (!val) return
  log(`Sending chat message: ${val}`)
  ws.sendRaw('chat', null, val)
  input.value = ''
})
