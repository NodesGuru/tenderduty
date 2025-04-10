/**
 * GridRenderer
 * Handles canvas drawing for the grid visualization
 */
import { GRID_CONFIG, BLOCK_STATUS, THEME } from './constants.js';

export class GridRenderer {
  constructor() {
    // Grid dimensions
    this.cellHeight = GRID_CONFIG.CELL_HEIGHT;
    this.cellWidth = GRID_CONFIG.CELL_WIDTH;
    this.textMaxWidth = GRID_CONFIG.TEXT_MAX_WIDTH;
    this.textWidth = GRID_CONFIG.TEXT_WIDTH;
    
    // Scale for high DPI displays
    this.scale = 1;
    
    // Theme properties
    this.textColor = THEME.DARK.TEXT_COLOR;
    this.signColorAlpha = THEME.DARK.SIGN_COLOR_ALPHA;
    this.isDark = true;
    
    // Canvas elements
    this.canvas = document.getElementById('canvas');
    this.legendCanvas = document.getElementById('legend');
    
    // Listen for theme changes
    document.addEventListener('themeChanged', (event) => {
      this.textColor = event.detail.textColor;
      this.signColorAlpha = event.detail.signColorAlpha;
      this.isDark = event.detail.isDark;
      
      // Redraw legend with new theme
      this.drawLegend();
    });
  }

  /**
   * Adjust canvas for device pixel ratio to ensure crisp rendering
   * @param {string} canvasId - Canvas element ID
   * @private
   */
  _fixDevicePixelRatio(canvasId) {
    const canvas = document.getElementById(canvasId);
    const dpi = window.devicePixelRatio;
    
    // Calculate scaled dimensions
    this.scaledCellHeight = this.cellHeight * dpi.valueOf();
    this.scaledCellWidth = this.cellWidth * dpi.valueOf();
    this.scaledTextMaxWidth = this.textMaxWidth * dpi.valueOf();
    this.scaledTextWidth = this.textWidth * dpi.valueOf();
    
    // Get computed dimensions
    const style = {
      height() {
        return +getComputedStyle(canvas).getPropertyValue('height').slice(0, -2);
      },
      width() {
        return +getComputedStyle(canvas).getPropertyValue('width').slice(0, -2);
      }
    };
    
    // Set canvas dimensions accounting for device pixel ratio
    canvas.setAttribute('width', style.width() * dpi);
    canvas.setAttribute('height', style.height() * dpi);
    
    // Save scale for future reference
    this.scale = dpi.valueOf();
  }

  /**
   * Draw the legend showing the meaning of different colors
   */
  drawLegend() {
    // Fix DPI for crisp rendering
    this._fixDevicePixelRatio('legend');
    
    const ctx = this.legendCanvas.getContext('2d');
    
    // Clear the canvas
    ctx.clearRect(0, 0, this.legendCanvas.width, this.legendCanvas.height);
    
    // Calculate appropriate sizes for the legend
    // Use much smaller values for the legend compared to the main grid
    const legendCellHeight = 8; // Even smaller height for legend items
    const legendCellWidth = 6;  // Even smaller width for legend items
    const scaledLegendCellHeight = legendCellHeight * this.scale;
    const scaledLegendCellWidth = legendCellWidth * this.scale;
    const fontSize = 7 * this.scale; // Match the grid labels size
    
    // Center vertically
    const verticalPosition = this.legendCanvas.height / 2;
    
    // Center the whole legend horizontally (calculate total width)
    const legendItems = [
      { text: "proposer", width: 45 },
      { text: "proposer/empty", width: 80 },
      { text: "signed", width: 40 },
      { text: "miss/precommit", width: 75 },
      { text: "miss/prevote", width: 65 },
      { text: "missed", width: 40 },
      { text: "no data", width: 40 }
    ];
    
    // Calculate spacings between items (consistent)
    const itemSpacing = 25;
    
    // Calculate total width needed
    const totalItemWidth = legendItems.reduce((acc, item) => acc + item.width, 0);
    const totalSpacingWidth = itemSpacing * (legendItems.length - 1);
    const totalColorBlockWidth = scaledLegendCellWidth * legendItems.length;
    const totalColorBlockSpacing = 4 * this.scale * legendItems.length;
    const totalLegendWidth = totalItemWidth + totalSpacingWidth + totalColorBlockWidth + totalColorBlockSpacing;
    
    // Start position (centered)
    const startOffset = (this.legendCanvas.width - totalLegendWidth) / 2;
    let offset = Math.max(startOffset, 10 * this.scale); // Ensure minimum margin
    
    // Set font once for all text
    ctx.font = `${fontSize}px sans-serif`;
    ctx.fillStyle = 'grey';
    
    // Draw each legend item
    const drawLegendItem = (text, gradientFn, additionalRender) => {
      // Draw colored block
      const grad = gradientFn(offset, verticalPosition, scaledLegendCellWidth, scaledLegendCellHeight);
      ctx.fillStyle = grad;
      ctx.fillRect(
        offset, 
        verticalPosition - scaledLegendCellHeight/2, 
        scaledLegendCellWidth, 
        scaledLegendCellHeight
      );
      
      // Execute any additional rendering for this item
      if (additionalRender) {
        additionalRender(offset, verticalPosition, scaledLegendCellWidth, scaledLegendCellHeight);
      }
      
      // Draw label
      ctx.fillStyle = 'grey';
      offset += scaledLegendCellWidth + 4 * this.scale;
      ctx.fillText(text, offset, verticalPosition + fontSize/3);
      
      // Move to next item
      const textWidth = ctx.measureText(text).width;
      offset += textWidth + itemSpacing * this.scale;
    };
    
    // Proposer
    drawLegendItem("proposer", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, 'rgb(123,255,66)');
      grad.addColorStop(0.3, 'rgb(240,255,128)');
      grad.addColorStop(0.8, 'rgb(169,250,149)');
      return grad;
    });
    
    // Proposer/empty
    drawLegendItem("proposer/empty", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, 'rgb(255,215,0)');
      grad.addColorStop(0.3, 'rgb(255,235,100)');
      grad.addColorStop(0.8, 'rgb(255,223,66)');
      return grad;
    });
    
    // Signed
    drawLegendItem("signed", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, 'rgba(0,0,0,0.2)');
      return grad;
    });
    
    // Miss/precommit
    drawLegendItem("miss/precommit", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, '#85c0f9');
      grad.addColorStop(0.7, '#85c0f9');
      grad.addColorStop(1, '#0b2641');
      return grad;
    });
    
    // Miss/prevote
    drawLegendItem("miss/prevote", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, '#381a34');
      grad.addColorStop(0.2, '#d06ec7');
      grad.addColorStop(1, '#d06ec7');
      return grad;
    });
    
    // Missed (with line through the middle)
    drawLegendItem("missed", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, '#8e4b26');
      grad.addColorStop(0.4, 'darkorange');
      return grad;
    }, (x, y, w, h) => {
      // Draw white line for missed blocks
      ctx.beginPath();
      ctx.moveTo(x + 1, y);
      ctx.lineTo(x + w - 1, y);
      ctx.closePath();
      ctx.strokeStyle = 'white';
      ctx.lineWidth = 0.5 * this.scale;
      ctx.stroke();
    });
    
    // No data
    drawLegendItem("no data", (x, y, w, h) => {
      const grad = ctx.createLinearGradient(x, y-h/2, x+w, y-h/2);
      grad.addColorStop(0, 'rgba(127,127,127,0.3)');
      return grad;
    });
  }

  /**
   * Draw the block series visualization
   * @param {Object} multiStates - The state data containing status information
   */
  drawSeries(multiStates) {
    if (!multiStates || !multiStates.Status || !multiStates.Status.length) {
      console.error('Invalid state data for grid rendering');
      return;
    }
    
    // Resize canvas based on data
    this.canvas.height = ((12 * this.scaledCellHeight * multiStates.Status.length) / 10) + 30;
    
    // Fix DPI for crisp rendering
    this._fixDevicePixelRatio('canvas');
    
    const ctx = this.canvas.getContext('2d');
    ctx.font = `${this.scale * 16}px sans-serif`;
    ctx.fillStyle = this.textColor;
    
    // Draw each status row
    for (let j = 0; j < multiStates.Status.length; j++) {
      // Draw chain name
      ctx.fillStyle = this.textColor;
      ctx.fillText(
        multiStates.Status[j].name, 
        5, 
        (j * this.scaledCellHeight) + (this.scaledCellHeight * 2) - 6, 
        this.scaledTextMaxWidth
      );
      
      // Draw block states
      for (let i = 0; i < multiStates.Status[j].blocks.length; i++) {
        let crossThrough = false;
        const grad = ctx.createLinearGradient(
          (i * this.scaledCellWidth) + this.scaledTextWidth, 
          (this.scaledCellHeight * j), 
          (i * this.scaledCellWidth) + this.scaledCellWidth + this.scaledTextWidth, 
          (this.scaledCellHeight * j)
        );
        
        // Set gradient based on block state
        switch (multiStates.Status[j].blocks[i]) {
          case BLOCK_STATUS.EMPTY_PROPOSED: // empty proposed
            grad.addColorStop(0, 'rgb(255,215,0)');
            grad.addColorStop(0.3, 'rgb(255,235,100)');
            grad.addColorStop(0.8, 'rgb(255,223,66)');
            break;
          case BLOCK_STATUS.PROPOSED: // proposed
            grad.addColorStop(0, 'rgb(123,255,66)');
            grad.addColorStop(0.3, 'rgb(240,255,128)');
            grad.addColorStop(0.8, 'rgb(169,250,149)');
            break;
          case BLOCK_STATUS.SIGNED: // signed
            if (j % 2 === 0) {
              grad.addColorStop(0, `rgba(0,0,0,${this.signColorAlpha})`);
              grad.addColorStop(0.9, `rgba(0,0,0,${this.signColorAlpha})`);
            } else {
              grad.addColorStop(0, `rgba(0,0,0,${this.signColorAlpha-0.3})`);
              grad.addColorStop(0.9, `rgba(0,0,0,${this.signColorAlpha-0.3})`);
            }
            grad.addColorStop(1, 'rgb(186,186,186)');
            break;
          case BLOCK_STATUS.PRECOMMIT_MISSED: // precommit not included
            grad.addColorStop(0, '#85c0f9');
            grad.addColorStop(0.8, '#85c0f9');
            grad.addColorStop(1, '#0b2641');
            break;
          case BLOCK_STATUS.PREVOTE_MISSED: // prevote not included
            grad.addColorStop(0, '#381a34');
            grad.addColorStop(0.2, '#d06ec7');
            grad.addColorStop(1, '#d06ec7');
            break;
          case BLOCK_STATUS.MISSED: // missed
            grad.addColorStop(0, '#c15600');
            crossThrough = true;
            break;
          default: // no data
            grad.addColorStop(0, 'rgba(127,127,127,0.3)');
        }
        
        // Draw block cell
        ctx.clearRect(
          (i * this.scaledCellWidth) + this.scaledTextWidth, 
          this.scaledCellHeight + (this.scaledCellHeight * j), 
          this.scaledCellWidth, 
          this.scaledCellHeight
        );
        ctx.fillStyle = grad;
        ctx.fillRect(
          (i * this.scaledCellWidth) + this.scaledTextWidth, 
          this.scaledCellHeight + (this.scaledCellHeight * j), 
          this.scaledCellWidth, 
          this.scaledCellHeight
        );
        
        // Draw line between blocks
        if (i > 0) {
          ctx.beginPath();
          ctx.moveTo(
            (i * this.scaledCellWidth) - this.scaledCellWidth + this.scaledTextWidth, 
            2 * this.scaledCellHeight + (this.scaledCellHeight * j) - 0.5
          );
          ctx.lineTo(
            (i * this.scaledCellWidth) + this.scaledTextWidth, 
            2 * this.scaledCellHeight + (this.scaledCellHeight * j) - 0.5
          );
          ctx.closePath();
          ctx.strokeStyle = 'rgb(51,51,51)';
          ctx.lineWidth = 0.5;
          ctx.stroke();
        }
        
        // Draw visual indicator for missed blocks
        if (crossThrough) {
          ctx.beginPath();
          ctx.moveTo(
            (i * this.scaledCellWidth) + this.scaledTextWidth + 1 + this.scaledCellWidth / 4, 
            (this.scaledCellHeight * j) + (this.scaledCellHeight * 2) - this.scaledCellHeight / 2
          );
          ctx.lineTo(
            (i * this.scaledCellWidth) + this.scaledTextWidth + this.scaledCellWidth - (this.scaledCellWidth / 4) - 1, 
            (this.scaledCellHeight * j) + (this.scaledCellHeight * 2) - this.scaledCellHeight / 2
          );
          ctx.closePath();
          ctx.strokeStyle = 'white';
          ctx.stroke();
        }
      }
    }
  }
} 