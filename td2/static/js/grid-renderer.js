/**
 * GridRenderer
 * Handles visualization for the block grid using modern DOM elements
 */
import { BLOCK_STATUS } from './constants.js';

export class GridRenderer {
  constructor() {
    this.gridContainer = document.getElementById('grid-container');
    
    // Debug: Check if grid container exists
    if (!this.gridContainer) {
      console.error('Grid container not found! Make sure grid-container element exists in HTML');
    } else {
      console.log('Grid container found:', this.gridContainer);
    }
    
    this.blocksPerRow = 500; // Number of blocks to show for each chain
    this.blockHeights = new Map(); // Track block heights for animation
  }

  /**
   * Draw the block grid visualization
   * @param {Object} data - The state data containing status information
   */
  drawSeries(data) {
    console.log('GridRenderer.drawSeries called with data:', data);
    
    if (!data || !data.Status || !data.Status.length) {
      console.error('Invalid state data for grid rendering');
      return;
    }
    
    if (!this.gridContainer) {
      console.error('Cannot render grid: grid-container element not found');
      // Try to get the container again, it might have loaded since initialization
      this.gridContainer = document.getElementById('grid-container');
      if (!this.gridContainer) {
        return;
      }
    }

    // Clear the grid container
    this.gridContainer.innerHTML = '';
    
    // Create a row for each chain
    data.Status.forEach((chainStatus, chainIndex) => {
      // Create row element
      const row = document.createElement('div');
      row.className = 'chain-row';
      
      // Add chain name label
      const label = document.createElement('div');
      label.className = 'chain-label';
      label.textContent = chainStatus.name;
      row.appendChild(label);
      
      // Create blocks container
      const blocksContainer = document.createElement('div');
      blocksContainer.className = 'blocks-container';
      
      // Check how blocks are stored in the status data
      // The original implementation may be using a different data structure
      let blockData = [];
      
      if (chainStatus.blocks && Array.isArray(chainStatus.blocks)) {
        console.log(`Chain ${chainStatus.name} has ${chainStatus.blocks.length} blocks as array`);
        blockData = chainStatus.blocks;
      } else if (typeof chainStatus.blocks === 'object') {
        // Convert object to array if needed
        console.log(`Chain ${chainStatus.name} has blocks as object, converting to array`);
        blockData = Object.values(chainStatus.blocks);
      } else {
        console.warn(`Chain ${chainStatus.name} has no blocks data or unsupported format`);
      }
      
      // Ensure we have some blocks to show even if data is missing
      if (blockData.length === 0) {
        console.log('No block data, creating placeholder blocks');
        blockData = new Array(50).fill(BLOCK_STATUS.NO_DATA);
      }
      
      // Add individual blocks
      blockData.forEach((blockStatus, blockIndex) => {
        const block = this._createBlockElement(blockStatus, chainIndex, blockIndex);
        blocksContainer.appendChild(block);
      });
      
      row.appendChild(blocksContainer);
      this.gridContainer.appendChild(row);
      
      // Also store the current block height for animation tracking
      if (chainStatus.chain_id && chainStatus.height) {
        this.blockHeights.set(chainStatus.chain_id, chainStatus.height);
      }
    });
    
    console.log('Grid rendering complete');
  }
  
  /**
   * Create a single block element
   * @param {number} status - Block status code
   * @param {number} chainIndex - Index of the chain
   * @param {number} blockIndex - Index of the block
   * @returns {HTMLElement} Block element
   * @private
   */
  _createBlockElement(status, chainIndex, blockIndex) {
    const block = document.createElement('div');
    block.className = 'block';
    
    // Convert status to number if it's a string
    const blockStatus = parseInt(status);
    
    // Apply status-specific class
    switch (blockStatus) {
      case BLOCK_STATUS.PROPOSED: // 4
        block.classList.add('status-proposed');
        break;
      case BLOCK_STATUS.EMPTY_PROPOSED: // 5
        block.classList.add('status-empty-proposed');
        break;
      case BLOCK_STATUS.SIGNED: // 3
        block.classList.add('status-signed');
        // Different styling for even/odd rows
        block.classList.add(chainIndex % 2 === 0 ? 'even' : 'odd');
        break;
      case BLOCK_STATUS.PRECOMMIT_MISSED: // 2
        block.classList.add('status-miss-precommit');
        break;
      case BLOCK_STATUS.PREVOTE_MISSED: // 1
        block.classList.add('status-miss-prevote');
        break;
      case BLOCK_STATUS.MISSED: // 0
        block.classList.add('status-missed');
        // Add white line for missed blocks
        const line = document.createElement('div');
        line.className = 'block-line';
        block.appendChild(line);
        break;
      default:
        console.log('Unknown block status:', status);
        block.classList.add('status-no-data');
    }
    
    return block;
  }
  
  /**
   * Update block height and determine if animation should be triggered
   * @param {string} chainId - Chain ID
   * @param {number} height - Block height
   * @returns {boolean} True if height changed and animation should be triggered
   */
  updateBlockHeight(chainId, height) {
    const previousHeight = this.blockHeights.get(chainId);
    const changed = previousHeight !== undefined && previousHeight !== height;
    
    // Update stored height
    this.blockHeights.set(chainId, height);
    
    // If height changed, update visual representation
    if (changed) {
      this.animateHeightChange(chainId, height);
    }
    
    return changed;
  }

  /**
   * Animate height change for a specific chain
   * @param {string} chainId - Chain ID
   * @param {number} newHeight - New block height
   */
  animateHeightChange(chainId, newHeight) {
    // Update height elements in the table (if they exist)
    const heightElements = document.querySelectorAll(`[data-chain="${chainId}"]`);
    console.log(`Animating height change for ${chainId} to ${newHeight}, found ${heightElements.length} elements`);
    
    heightElements.forEach(element => {
      // Update text
      element.textContent = newHeight;
      
      // Apply animation
      element.classList.remove('block-height-change');
      void element.offsetWidth; // Force reflow to restart animation
      element.classList.add('block-height-change');
    });
  }

  /**
   * No need to explicitly draw the legend as it's rendered through HTML/CSS
   */
  drawLegend() {
    // Legend is now built with HTML/CSS, no additional rendering needed
    console.log('drawLegend called - no action needed as legend is built with HTML/CSS');
  }
} 