// Global state
let chart = null;
let candlestickSeries = null;
let ws = null;
let token = localStorage.getItem('token');

// Constants
const API_BASE = 'http://localhost:8080';
const WS_BASE = 'ws://localhost:8080';

// Initialize the TradingView chart
function initChart() {
    const chartContainer = document.getElementById('chart');
    chart = LightweightCharts.createChart(chartContainer, {
        width: chartContainer.clientWidth,
        height: chartContainer.clientHeight,
        layout: {
            background: { color: '#0a0a0a' },
            textColor: '#d1d4dc',
        },
        grid: {
            vertLines: { color: '#1a1a1a' },
            horzLines: { color: '#1a1a1a' },
        },
        crosshair: {
            mode: LightweightCharts.CrosshairMode.Normal,
        },
        rightPriceScale: {
            borderColor: '#1a1a1a',
        },
        timeScale: {
            borderColor: '#1a1a1a',
        },
    });

    candlestickSeries = chart.addCandlestickSeries({
        upColor: '#45b26b',
        downColor: '#ef466f',
        borderVisible: false,
        wickUpColor: '#45b26b',
        wickDownColor: '#ef466f',
    });

    // Handle window resize
    window.addEventListener('resize', () => {
        chart.applyOptions({
            width: chartContainer.clientWidth,
            height: chartContainer.clientHeight,
        });
    });
}

// WebSocket connection
function connectWebSocket() {
    ws = new WebSocket(`${WS_BASE}/ws`);
    
    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        updateOrderBook(data);
    };

    ws.onclose = () => {
        setTimeout(connectWebSocket, 1000); // Reconnect after 1 second
    };
}

// Update order book visualization
function updateOrderBook(data) {
    if (!data.buy_orders || !data.sell_orders) return;

    const timestamp = new Date().getTime() / 1000;
    const buyOrders = data.buy_orders.sort((a, b) => b.price - a.price);
    const sellOrders = data.sell_orders.sort((a, b) => a.price - b.price);

    if (buyOrders.length > 0 && sellOrders.length > 0) {
        const midPrice = (buyOrders[0].price + sellOrders[0].price) / 2;
        candlestickSeries.update({
            time: timestamp,
            open: midPrice,
            high: sellOrders[0].price,
            low: buyOrders[0].price,
            close: midPrice,
        });
    }
}

// API calls
async function login(username, password) {
    try {
        const response = await fetch(`${API_BASE}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (!response.ok) throw new Error('Login failed');

        const data = await response.json();
        token = data.token;
        localStorage.setItem('token', token);
        
        document.getElementById('login-section').classList.add('hidden');
        document.getElementById('trading-section').classList.remove('hidden');
        
        initChart();
        connectWebSocket();
        fetchOrders();
    } catch (error) {
        alert('Login failed: ' + error.message);
    }
}

async function placeOrder(type, price, quantity) {
    try {
        const response = await fetch(`${API_BASE}/orders`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`,
            },
            body: JSON.stringify({ type, price: parseFloat(price), quantity: parseFloat(quantity) }),
        });

        if (!response.ok) throw new Error('Failed to place order');
        
        fetchOrders(); // Refresh orders list
    } catch (error) {
        alert('Failed to place order: ' + error.message);
    }
}

async function cancelOrder(orderId) {
    try {
        const response = await fetch(`${API_BASE}/orders/${orderId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${token}`,
            },
        });

        if (!response.ok) throw new Error('Failed to cancel order');
        
        fetchOrders(); // Refresh orders list
    } catch (error) {
        alert('Failed to cancel order: ' + error.message);
    }
}

async function fetchOrders() {
    try {
        const response = await fetch(`${API_BASE}/orders`, {
            headers: {
                'Authorization': `Bearer ${token}`,
            },
        });

        if (!response.ok) throw new Error('Failed to fetch orders');

        const orders = await response.json();
        updateOrdersTable(orders);
    } catch (error) {
        console.error('Failed to fetch orders:', error);
    }
}

function updateOrdersTable(orders) {
    const tbody = document.getElementById('orders-body');
    tbody.innerHTML = '';

    orders.forEach(order => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${order.id}</td>
            <td class="${order.type.toLowerCase()}">${order.type}</td>
            <td>${order.price}</td>
            <td>${order.quantity}</td>
            <td>${order.status}</td>
            <td>
                <button class="cancel-btn" onclick="cancelOrder('${order.id}')">Cancel</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

// Event Listeners
document.getElementById('login-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;
    login(username, password);
});

document.getElementById('order-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const type = document.getElementById('order-type').value;
    const price = document.getElementById('price').value;
    const quantity = document.getElementById('quantity').value;
    placeOrder(type, price, quantity);
});

// Check if user is already logged in
if (token) {
    document.getElementById('login-section').classList.add('hidden');
    document.getElementById('trading-section').classList.remove('hidden');
    initChart();
    connectWebSocket();
    fetchOrders();
} 