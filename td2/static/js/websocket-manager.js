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
    const wsProtocol = location.protocol === 'https:' ? 'wss://' : 'ws://';
    return `${wsProtocol}${location.host}/${API.WEBSOCKET}`;
    
    // Uncomment for local development with hardcoded URL
    // return `ws://127.0.0.1:8888/${API.WEBSOCKET}`;
  }

  /**
   * Connect to WebSocket server
   */
  connect() {
    if (this.socket) {
      this.disconnect();
    }

    try {
      const url = this._getWebSocketUrl();
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
    setTimeout(() => {
      console.log('Attempting to reconnect WebSocket...');
      this.connect();
    }, this.reconnectTimeout);
  }
} 