// Global state
let chart = null;
let candlestickSeries = null;
let ws = null;
let token = localStorage.getItem('token');

// Constants
const API_BASE = 'http://localhost:8080';
const WS_BASE = 'ws://localhost:8080';

// Generate mock candlestick data
function generateMockCandlestickData() {
    const data = [];
    const now = Math.floor(Date.now() / 1000);
    let basePrice = 30000; // Initial BTC price
    
    for (let i = 0; i < 100; i++) {
        const time = now - (100 - i) * 3600; // hourly candles, going back 100 hours
        const open = basePrice;
        const high = open + open * (Math.random() * 0.02); // up to 2% higher
        const low = open - open * (Math.random() * 0.02); // up to 2% lower
        const close = low + Math.random() * (high - low); // random close between high and low
        
        data.push({
            time,
            open,
            high,
            low,
            close,
        });
        
        // Set next base price to the last close
        basePrice = close;
    }
    
    return data;
}

// Generate mock order book data
function generateMockOrderBookData() {
    const lastPrice = 30000 + Math.random() * 2000;
    const bids = [];
    const asks = [];
    
    // Generate 10 bids (buy orders) below the last price
    for (let i = 0; i < 10; i++) {
        const price = lastPrice - (i * 50) - Math.random() * 10;
        const quantity = 0.1 + Math.random() * 2; // Between 0.1 and 2.1 BTC
        bids.push({
            price: price.toFixed(2),
            quantity: quantity.toFixed(3)
        });
    }
    
    // Generate 10 asks (sell orders) above the last price
    for (let i = 0; i < 10; i++) {
        const price = lastPrice + (i * 50) + Math.random() * 10;
        const quantity = 0.1 + Math.random() * 2; // Between 0.1 and 2.1 BTC
        asks.push({
            price: price.toFixed(2),
            quantity: quantity.toFixed(3)
        });
    }
    
    return { bids, asks };
}

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
            scaleMargins: {
                top: 0.1,
                bottom: 0.1,
            },
        },
        timeScale: {
            borderColor: '#1a1a1a',
            timeVisible: true,
            secondsVisible: false,
        },
    });

    candlestickSeries = chart.addCandlestickSeries({
        upColor: '#45b26b',
        downColor: '#ef466f',
        borderVisible: false,
        wickUpColor: '#45b26b',
        wickDownColor: '#ef466f',
    });

    // Load mock data
    const mockData = generateMockCandlestickData();
    candlestickSeries.setData(mockData);

    // Handle window resize
    window.addEventListener('resize', () => {
        if (chart) {
            chart.applyOptions({
                width: chartContainer.clientWidth,
                height: chartContainer.clientHeight,
            });
        }
    });
    
    // Initially populate the order book with mock data
    updateOrderBookUI(generateMockOrderBookData());
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

// Update order book based on WebSocket data
function updateOrderBook(data) {
    if (!data.buy_orders || !data.sell_orders) return;

    const timestamp = Math.floor(Date.now() / 1000);
    const buyOrders = data.buy_orders.sort((a, b) => b.price - a.price);
    const sellOrders = data.sell_orders.sort((a, b) => a.price - b.price);

    // Format for UI display
    const bids = buyOrders.map(order => ({
        price: order.price.toFixed(2),
        quantity: order.quantity.toFixed(3)
    }));
    
    const asks = sellOrders.map(order => ({
        price: order.price.toFixed(2),
        quantity: order.quantity.toFixed(3)
    }));

    // Update UI
    updateOrderBookUI({ bids, asks });

    // Update chart with latest price if available
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

// Update the order book UI
function updateOrderBookUI(data) {
    const { bids, asks } = data;
    
    // Update asks (sell orders)
    const asksBody = document.getElementById('asks-body');
    asksBody.innerHTML = '';
    
    // Sort asks from highest to lowest
    asks.sort((a, b) => b.price - a.price).forEach(ask => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${ask.price}</td>
            <td>${ask.quantity}</td>
        `;
        asksBody.appendChild(tr);
    });
    
    // Update bids (buy orders)
    const bidsBody = document.getElementById('bids-body');
    bidsBody.innerHTML = '';
    
    // Sort bids from highest to lowest
    bids.sort((a, b) => b.price - a.price).forEach(bid => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${bid.price}</td>
            <td>${bid.quantity}</td>
        `;
        bidsBody.appendChild(tr);
    });
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

// Update order button color based on type selection
document.getElementById('order-type').addEventListener('change', (e) => {
    const placeOrderBtn = document.getElementById('place-order-btn');
    if (e.target.value === 'buy') {
        placeOrderBtn.style.backgroundColor = 'var(--accent-buy)';
    } else {
        placeOrderBtn.style.backgroundColor = 'var(--accent-sell)';
    }
});

// Check if user is already logged in
if (token) {
    document.getElementById('login-section').classList.add('hidden');
    document.getElementById('trading-section').classList.remove('hidden');
    initChart();
    connectWebSocket();
    fetchOrders();
    
    // Set initial button color
    const orderType = document.getElementById('order-type');
    const placeOrderBtn = document.getElementById('place-order-btn');
    if (orderType.value === 'buy') {
        placeOrderBtn.style.backgroundColor = 'var(--accent-buy)';
    } else {
        placeOrderBtn.style.backgroundColor = 'var(--accent-sell)';
    }
} 