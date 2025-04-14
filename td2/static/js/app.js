/**
 * Tenderduty Dashboard
 * Main application entry point
 */

// Import modules
import { ThemeManager } from './theme-manager.js';
import { DataService } from './data-service.js';
import { GridRenderer } from './grid-renderer.js';
import { TableRenderer } from './table-renderer.js';
import { LogManager } from './log-manager.js';
import { WebSocketManager } from './websocket-manager.js';
import { WS_MESSAGE_TYPES } from './constants.js';

class App {
  constructor() {
    // Initialize modules
    this.themeManager = new ThemeManager();
    this.dataService = new DataService();
    this.gridRenderer = new GridRenderer();
    this.tableRenderer = new TableRenderer();
    this.logManager = new LogManager();
    this.wsManager = new WebSocketManager();
    
    // Connect components
    console.log('Connecting TableRenderer to GridRenderer');
    this.tableRenderer.setGridRenderer(this.gridRenderer);
    
    // Register event listeners
    this._registerEventListeners();
  }

  /**
   * Register event handlers for the application
   */
  _registerEventListeners() {
    // Initialize theme toggle button
    document.querySelector('.theme-toggle').addEventListener('click', () => {
      this.themeManager.toggleTheme();
    });

    // Handle websocket messages
    this.wsManager.onMessage((message) => {
      const msg = JSON.parse(message.data);
      console.log('WebSocket message received in app.js:', msg.msgType);
      
      if (msg.msgType === WS_MESSAGE_TYPES.LOG) {
        this.logManager.addLogMessage(msg.ts, msg.msg);
      } else if (msg.msgType === WS_MESSAGE_TYPES.UPDATE && document.visibilityState !== "hidden") {
        console.log('Updating UI with new data');
        this.tableRenderer.updateTable(msg);
        this.gridRenderer.drawSeries(msg);
      } else {
        console.log('Message type not handled or tab not visible:', msg.msgType, document.visibilityState);
      }
    });
    
    // Listen for visibility changes to update UI when tab becomes visible
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState !== 'hidden') {
        console.log('Tab became visible, updating log display');
        this.logManager._updateLogDisplay();
      }
    });
  }

  /**
   * Initialize the application
   */
  async init() {
    try {
      console.log('Initializing application');
      
      // Legend is now built with HTML/CSS, no need to explicitly draw it
      // Keeping the gridRenderer.drawLegend() method for API compatibility
      
      // Load initial state
      console.log('Loading initial state...');
      const state = await this.dataService.loadState();
      console.log('Initial state loaded:', state ? 'success' : 'failed');
      
      // Initialize UI with state data
      if (state) {
        console.log('Updating UI with initial state');
        this.tableRenderer.updateTable(state);
        this.gridRenderer.drawSeries(state);
        
        // Load initial logs if available
        if (state.logs) {
          console.log('Loading initial logs');
          this.logManager.loadInitialLogs(state.logs);
        }
      }
      
      // Connect to websocket for real-time updates
      console.log('Connecting to WebSocket for real-time updates');
      this.wsManager.connect();
      
      console.log('Tenderduty Dashboard initialized');
    } catch (error) {
      console.error('Failed to initialize application:', error);
    }
  }
}

// Initialize the application when the DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  console.log('DOM loaded, initializing app');
  const app = new App();
  app.init();
}); 