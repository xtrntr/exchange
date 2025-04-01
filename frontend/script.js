// Global state
let chart = null;
let candlestickSeries = null;
let ws = null;
let token = localStorage.getItem('token');

// Constants
const API_BASE = 'http://localhost:8080';
const WS_BASE = 'ws://localhost:8080';

// Add new function to fetch and process trade data
async function fetchTradeData() {
    try {
        const response = await fetch(`${API_BASE}/trades`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });
        
        if (!response.ok) throw new Error('Failed to fetch trades');
        
        const trades = await response.json();
        return processTrades(trades);
    } catch (error) {
        console.error('Error fetching trade data:', error);
        return [];
    }
}

// Process trades into candlestick format
function processTrades(trades) {
    if (!Array.isArray(trades) || trades.length === 0) {
        console.log('No trades available');
        return [];
    }

    // Group trades by minute
    const candleMap = new Map();
    
    trades.forEach(trade => {
        if (!trade || !trade.executed_at || !trade.price) {
            console.warn('Invalid trade data:', trade);
            return;
        }

        const timestamp = new Date(trade.executed_at);
        // Round to nearest minute
        timestamp.setSeconds(0, 0);
        const minuteKey = timestamp.getTime() / 1000;
        
        if (!candleMap.has(minuteKey)) {
            candleMap.set(minuteKey, {
                time: minuteKey,
                open: trade.price,
                high: trade.price,
                low: trade.price,
                close: trade.price,
                trades: [trade]
            });
        } else {
            const candle = candleMap.get(minuteKey);
            candle.high = Math.max(candle.high, trade.price);
            candle.low = Math.min(candle.low, trade.price);
            candle.close = trade.price;
            candle.trades.push(trade);
        }
    });
    
    return Array.from(candleMap.values());
}

// Initialize the TradingView chart with retry mechanism
async function initChart(retryCount = 0) {
    console.log('Initializing chart...');
    const MAX_RETRIES = 5;
    
    if (retryCount >= MAX_RETRIES) {
        console.error('Failed to initialize chart after max retries');
        return;
    }

    // Wait for the library to load
    if (typeof LightweightCharts === 'undefined') {
        console.log(`Waiting for TradingView library... (attempt ${retryCount + 1})`);
        setTimeout(() => initChart(retryCount + 1), 1000);
        return;
    }

    const chartContainer = document.getElementById('chart');
    if (!chartContainer) {
        console.error('Chart container not found');
        return;
    }

    try {
        console.log('Creating chart...');
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
                timeVisible: true,
                secondsVisible: false,
            },
        });

        console.log('Adding candlestick series...');
        candlestickSeries = chart.addCandlestickSeries({
            upColor: '#45b26b',
            downColor: '#ef466f',
            borderVisible: false,
            wickUpColor: '#45b26b',
            wickDownColor: '#ef466f',
        });

        // Get real trade data
        const tradeData = await fetchTradeData();
        if (tradeData.length > 0) {
            console.log('Setting trade data');
            candlestickSeries.setData(tradeData);
        } else {
            console.log('No trades available');
        }

        // Set up periodic updates
        setInterval(async () => {
            const newData = await fetchTradeData();
            if (newData.length > 0) {
                candlestickSeries.setData(newData);
            }
        }, 60000); // Update every minute

        // Handle window resize
        window.addEventListener('resize', () => {
            if (chart) {
                chart.applyOptions({
                    width: chartContainer.clientWidth,
                    height: chartContainer.clientHeight,
                });
            }
        });
        
        // Connect to WebSocket for real-time updates
        connectWebSocket();
        
        console.log('Chart initialized successfully');
    } catch (error) {
        console.error('Chart initialization error:', error);
        setTimeout(() => initChart(retryCount + 1), 1000);
    }
}

// WebSocket connection
function connectWebSocket() {
    try {
        console.log('Connecting to WebSocket...');
        ws = new WebSocket(`${WS_BASE}/ws`);
        
        ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                console.log('WebSocket message received:', data); // Debug log
                updateOrderBook(data);
            } catch (error) {
                console.error('Error processing WebSocket message:', error);
            }
        };

        ws.onclose = (event) => {
            console.log('WebSocket connection closed:', event.code, event.reason);
            setTimeout(() => {
                console.log('Attempting to reconnect WebSocket...');
                connectWebSocket();
            }, 1000);
        };

        ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        ws.onopen = () => {
            console.log('WebSocket connection established');
        };
    } catch (error) {
        console.error('Error creating WebSocket connection:', error);
        setTimeout(connectWebSocket, 1000);
    }
}

// Update order book based on WebSocket data
function updateOrderBook(data) {
    if (!data || !data.buy_orders || !data.sell_orders) {
        console.warn('Invalid orderbook data:', data);
        return;
    }

    const timestamp = Math.floor(Date.now() / 1000);
    const buyOrders = data.buy_orders
        .filter(order => order && typeof order.price === 'number' && typeof order.quantity === 'number')
        .sort((a, b) => b.price - a.price);
    const sellOrders = data.sell_orders
        .filter(order => order && typeof order.price === 'number' && typeof order.quantity === 'number')
        .sort((a, b) => a.price - b.price);

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
        if (candlestickSeries) {
            candlestickSeries.update({
                time: timestamp,
                open: midPrice,
                high: sellOrders[0].price,
                low: buyOrders[0].price,
                close: midPrice,
            });
        }
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

// Initialize everything in sequence
async function initializeTrading() {
    try {
        // Initialize chart first
        await initChart();
        
        // Fetch initial order book data
        const orderBookResponse = await fetch(`${API_BASE}/orderbook`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });
        
        if (orderBookResponse.ok) {
            const orderBookData = await orderBookResponse.json();
            updateOrderBook(orderBookData);
        }
        
        // Fetch initial trades data
        const tradesResponse = await fetch(`${API_BASE}/trades`, {
            headers: {
                'Authorization': `Bearer ${token}`
            }
        });
        
        if (tradesResponse.ok) {
            const trades = await tradesResponse.json();
            const candleData = processTrades(trades);
            if (candlestickSeries && candleData.length > 0) {
                candlestickSeries.setData(candleData);
            }
        }
        
        // Connect WebSocket for real-time updates
        connectWebSocket();
        
        // Fetch user's orders
        await fetchOrders();
        
        console.log('Trading interface initialized successfully');
    } catch (error) {
        console.error('Error initializing trading interface:', error);
    }
}

// Update the login function to use the new initialization sequence
async function login(username, password) {
    try {
        const response = await fetch(`${API_BASE}/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Login failed');
        }

        const data = await response.json();
        token = data.token;
        localStorage.setItem('token', token);
        
        // Show trading section
        document.getElementById('login-section').classList.add('hidden');
        document.getElementById('trading-section').classList.remove('hidden');
        
        // Wait for DOM update
        await new Promise(resolve => setTimeout(resolve, 100));
        
        // Initialize trading interface
        await initializeTrading();
    } catch (error) {
        console.error('Login error:', error);
        alert('Login failed: ' + error.message);
    }
}

// Update the orders table function to handle missing data gracefully
function updateOrdersTable(orders) {
    const tbody = document.getElementById('orders-body');
    if (!tbody) {
        console.error('Orders table body not found');
        return;
    }
    
    tbody.innerHTML = '';

    if (!Array.isArray(orders)) {
        console.error('Invalid orders data:', orders);
        return;
    }

    if (orders.length === 0) {
        const tr = document.createElement('tr');
        tr.innerHTML = '<td colspan="6" style="text-align: center;">No orders found</td>';
        tbody.appendChild(tr);
        return;
    }

    orders.forEach(order => {
        if (!order || typeof order !== 'object') {
            console.error('Invalid order:', order);
            return;
        }

        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${order.ID || order.id || 'N/A'}</td>
            <td class="${(order.Type || order.type || '').toLowerCase()}">${order.Type || order.type || 'N/A'}</td>
            <td>${typeof order.Price === 'number' ? order.Price.toFixed(2) : (typeof order.price === 'number' ? order.price.toFixed(2) : 'N/A')}</td>
            <td>${typeof order.Quantity === 'number' ? order.Quantity.toFixed(3) : (typeof order.quantity === 'number' ? order.quantity.toFixed(3) : 'N/A')}</td>
            <td>${order.Status || order.status || 'N/A'}</td>
            <td>
                ${(order.Status || order.status) === 'open' ? 
                    `<button class="cancel-btn" onclick="cancelOrder('${order.ID || order.id}')">Cancel</button>` : 
                    ''}
            </td>
        `;
        tbody.appendChild(tr);
    });
}

// Update the fetchOrders function to handle errors better
async function fetchOrders() {
    try {
        const response = await fetch(`${API_BASE}/orders`, {
            headers: {
                'Authorization': `Bearer ${token}`,
            },
        });

        if (!response.ok) {
            throw new Error(`Failed to fetch orders: ${response.status} ${response.statusText}`);
        }

        const data = await response.json();
        console.log('Orders response:', data); // Debug log
        
        // Handle both array and object responses
        const orders = Array.isArray(data) ? data : [];
        
        if (orders.length === 0) {
            console.log('No orders found');
        }

        updateOrdersTable(orders);
    } catch (error) {
        console.error('Failed to fetch orders:', error);
    }
}

// Update the placeOrder function to show more feedback
async function placeOrder(type, price, quantity) {
    try {
        console.log('Placing order:', { type, price, quantity });
        
        const response = await fetch(`${API_BASE}/orders`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${token}`,
            },
            body: JSON.stringify({ 
                type: type.toLowerCase(), 
                price: parseFloat(price), 
                quantity: parseFloat(quantity) 
            }),
        });

        if (!response.ok) {
            const errorData = await response.json();
            throw new Error(errorData.error || 'Failed to place order');
        }

        const result = await response.json();
        console.log('Order placed successfully:', result);
        
        // Show success message
        alert('Order placed successfully!');
        
        // Refresh orders list
        fetchOrders();
    } catch (error) {
        console.error('Order placement error:', error);
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
    
    // Initialize everything in sequence
    Promise.resolve().then(async () => {
        try {
            await initChart();
            connectWebSocket();
            await fetchOrders();
            console.log('Trading interface initialized successfully');
        } catch (error) {
            console.error('Error initializing trading interface:', error);
        }
    });
    
    // Set initial button color
    const orderType = document.getElementById('order-type');
    const placeOrderBtn = document.getElementById('place-order-btn');
    if (orderType && placeOrderBtn) {
        if (orderType.value === 'buy') {
            placeOrderBtn.style.backgroundColor = 'var(--accent-buy)';
        } else {
            placeOrderBtn.style.backgroundColor = 'var(--accent-sell)';
        }
    }
} 