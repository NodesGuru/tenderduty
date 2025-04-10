/**
 * ThemeManager
 * Handles theme switching between light and dark modes
 */
import { THEME } from './constants.js';

export class ThemeManager {
  constructor() {
    this.isDark = true;
    this.textColor = THEME.DARK.TEXT_COLOR;
    this.signColorAlpha = THEME.DARK.SIGN_COLOR_ALPHA;
    
    // Elements that need theme switching
    this.elements = {
      body: document.body,
      canvasDiv: document.getElementById('canvasDiv'),
      tableDiv: document.getElementById('tableDiv'),
      legendContainer: document.getElementById('legendContainer'),
      logs: document.getElementById('logs')
    };
  }

  /**
   * Toggle between light and dark themes
   * @returns {Object} Theme configuration with updated colors
   */
  toggleTheme() {
    this.isDark = !this.isDark;
    
    if (this.isDark) {
      this._applyDarkTheme();
    } else {
      this._applyLightTheme();
    }
    
    // Dispatch a custom event for other components to react to theme change
    const event = new CustomEvent('themeChanged', {
      detail: {
        isDark: this.isDark,
        textColor: this.textColor,
        signColorAlpha: this.signColorAlpha
      }
    });
    document.dispatchEvent(event);
    
    return {
      isDark: this.isDark,
      textColor: this.textColor,
      signColorAlpha: this.signColorAlpha
    };
  }

  /**
   * Apply dark theme to elements
   * @private
   */
  _applyDarkTheme() {
    this.textColor = THEME.DARK.TEXT_COLOR;
    this.signColorAlpha = THEME.DARK.SIGN_COLOR_ALPHA;
    
    this.elements.body.className = THEME.DARK.BODY_CLASS;
    this.elements.canvasDiv.className = `uk-width-expand uk-overflow-auto ${THEME.DARK.CONTAINER_CLASS}`;
    this.elements.tableDiv.className = `uk-padding-small uk-text-small ${THEME.DARK.CONTAINER_CLASS} uk-overflow-auto`;
    this.elements.legendContainer.className = `uk-nav-center ${THEME.DARK.CONTAINER_CLASS} uk-padding-remove`;
    this.elements.logs.style = THEME.DARK.LOG_STYLE;
  }

  /**
   * Apply light theme to elements
   * @private
   */
  _applyLightTheme() {
    this.textColor = THEME.LIGHT.TEXT_COLOR;
    this.signColorAlpha = THEME.LIGHT.SIGN_COLOR_ALPHA;
    
    this.elements.body.className = THEME.LIGHT.BODY_CLASS;
    this.elements.canvasDiv.className = `uk-width-expand uk-overflow-auto ${THEME.LIGHT.CONTAINER_CLASS}`;
    this.elements.tableDiv.className = `uk-padding-small uk-text-small ${THEME.LIGHT.CONTAINER_CLASS} uk-overflow-auto`;
    this.elements.legendContainer.className = `uk-nav-center ${THEME.LIGHT.CONTAINER_CLASS} uk-padding-remove`;
    this.elements.logs.style = THEME.LIGHT.LOG_STYLE;
  }

  /**
   * Get current theme configuration
   * @returns {Object} Current theme configuration
   */
  getThemeConfig() {
    return {
      isDark: this.isDark,
      textColor: this.textColor,
      signColorAlpha: this.signColorAlpha
    };
  }
} 