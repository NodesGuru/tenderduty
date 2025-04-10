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
      
      if (msg.msgType === WS_MESSAGE_TYPES.LOG) {
        this.logManager.addLogMessage(msg.ts, msg.msg);
      } else if (msg.msgType === WS_MESSAGE_TYPES.UPDATE && document.visibilityState !== "hidden") {
        this.tableRenderer.updateTable(msg);
        this.gridRenderer.drawSeries(msg);
      }
    });
  }

  /**
   * Initialize the application
   */
  async init() {
    try {
      // Draw the legend
      this.gridRenderer.drawLegend();
      
      // Load initial state
      const state = await this.dataService.loadState();
      
      // Initialize UI with state data
      if (state) {
        this.tableRenderer.updateTable(state);
        this.gridRenderer.drawSeries(state);
        
        // Load initial logs if available
        if (state.logs) {
          this.logManager.loadInitialLogs(state.logs);
        }
      }
      
      // Connect to websocket for real-time updates
      this.wsManager.connect();
      
      console.log('Tenderduty Dashboard initialized');
    } catch (error) {
      console.error('Failed to initialize application:', error);
    }
  }
}

// Initialize the application when the DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  const app = new App();
  app.init();
}); 