/**
 * WebSocketManager
 * Handles WebSocket connection and message processing
 */
import { API, WS_MESSAGE_TYPES } from './constants.js';

export class WebSocketManager {
  constructor() {
    this.socket = null;
    this.messageHandlers = [];
    this.isConnected = false;
    this.reconnectTimeout = 3000; // 3 seconds
  }

  /**
   * Register a message handler function
   * @param {Function} handler - Function to handle incoming messages
   */
  onMessage(handler) {
    if (typeof handler === 'function') {
      this.messageHandlers.push(handler);
    }
  }

  /**
   * Process incoming messages and distribute to all handlers
   * @param {MessageEvent} event - WebSocket message event
   * @private
   */
  _processMessage(event) {
    // Debug: Log received messages
    console.log('WebSocket message received:', event.data);
    
    try {
      // Parse the message to check its structure
      const data = JSON.parse(event.data);
      console.log('Parsed message:', data);
      console.log('Message type:', data.msgType);
    } catch (error) {
      console.error('Error parsing message:', error);
    }
    
    this.messageHandlers.forEach(handler => {
      try {
        handler(event);
      } catch (error) {
        console.error('Error in message handler:', error);
      }
    });
  }

  /**
   * Get WebSocket URL based on current page protocol and host
   * @returns {string} WebSocket URL
   * @private
   */
  _getWebSocketUrl() {
    // Try both options to ensure compatibility with the original implementation
    const wsProtocol = location.protocol === 'https:' ? 'wss://' : 'ws://';
    
    // First try the original URL format
    const wsUrl = `${wsProtocol}${location.host}/ws`;
    console.log('WebSocket URL:', wsUrl);
    
    // For local development, uncomment the line below
    return 'ws://127.0.0.1:8888/ws';
    
    return wsUrl;
  }

  /**
   * Connect to WebSocket server
   */
  connect() {
    if (this.socket && this.isConnected) {
      console.log('Already connected to WebSocket');
      return;
    }
    
    if (this.socket) {
      this.disconnect();
    }

    try {
      const url = this._getWebSocketUrl();
      console.log('Connecting to WebSocket at:', url);
      this.socket = new WebSocket(url);
      
      // Set up event handlers
      this.socket.addEventListener('message', (event) => this._processMessage(event));
      
      this.socket.addEventListener('open', () => {
        console.log('WebSocket connection established');
        this.isConnected = true;
      });
      
      this.socket.addEventListener('close', (event) => {
        console.log('WebSocket connection closed, retrying...', event.reason);
        this.isConnected = false;
        this._scheduleReconnect();
      });
      
      this.socket.addEventListener('error', (error) => {
        console.error('WebSocket error:', error);
        this.isConnected = false;
      });
      
    } catch (error) {
      console.error('Failed to connect to WebSocket:', error);
      this._scheduleReconnect();
    }
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect() {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
      this.isConnected = false;
    }
  }

  /**
   * Schedule reconnection attempt
   * @private
   */
  _scheduleReconnect() {
    console.log(`Scheduling reconnect in ${this.reconnectTimeout}ms`);
    setTimeout(() => {
      console.log('Attempting to reconnect WebSocket...');
      this.connect();
    }, this.reconnectTimeout);
  }
} 