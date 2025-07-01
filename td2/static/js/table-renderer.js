/**
 * TableRenderer
 * Handles rendering and updating of the status table
 */
export class TableRenderer {
  constructor() {
    this.statusTable = document.getElementById("statusTable");
    this.gridRenderer = null; // Will be set by the app

    // Listen for theme changes to potentially update table styles
    document.addEventListener("themeChanged", () => {
      // Future implementation: update table styles based on theme
    });
  }

  /**
   * Set reference to GridRenderer for height updates
   * @param {GridRenderer} gridRenderer - Reference to grid renderer
   */
  setGridRenderer(gridRenderer) {
    this.gridRenderer = gridRenderer;
  }

  /**
   * Create HTML markup for status indicator and potential modal
   * @param {Object} status - Status data for a chain
   * @returns {string} HTML markup for status indicator and modal
   * @private
   */
  _createStatusIndicator(status) {
    let statusClass = "";
    let statusText = "";
    let toggleAttribute = "";
    let modalHtml = "";
    const modalId = `modal-center-${_.escape(status.name)}`;

    // Create modal first if there is a last_error
    if (status.last_error !== "") {
      modalHtml = `
        <div id="${modalId}" class="uk-flex-top" uk-modal>
            <div class="uk-modal-dialog uk-modal-body uk-margin-auto-vertical uk-background-secondary">
                <button class="uk-modal-close-default" type="button" uk-close></button>
                <pre class=" uk-background-secondary" style="color: white">${_.escape(status.last_error)}</pre>
            </div>
        </div>
      `;
    }

    if (status.active_alerts > 0) {
      statusClass = "status-indicator-yellow";
      statusText = `${_.escape(status.active_alerts)} active issues`;
      // Make yellow indicator clickable only if there's an error modal to show
      if (status.last_error !== "") {
        toggleAttribute = `uk-toggle="target: #${modalId}"`;
      }
    } else {
      // Determine base status when no active alerts
      if (status.tombstoned) {
        statusClass = "status-indicator-red";
        statusText = "Tombstoned";
      } else if (status.jailed) {
        statusClass = "status-indicator-orange";
        statusText = "Jailed";
      } else if (status.bonded) {
        statusClass = "status-indicator-green";
        statusText = "Bonded (Active)";
      } else {
        // Includes 'not connected' case if moniker check is removed upstream
        statusClass = "status-indicator-gray";
        statusText = "Inactive";
      }

      // Make base status indicator clickable only if there's an error modal
      if (status.last_error !== "") {
        toggleAttribute = `uk-toggle="target: #${modalId}"`;
      }
    }

    // Handle 'not connected' separately for status text/class if needed, though covered by 'Inactive'
    if (status.moniker === "not connected") {
      statusClass = "status-indicator-gray";
      statusText = "Unknown Status";
      // Prevent clicking if not connected, even if last_error exists?
      // toggleAttribute = ''; // Uncomment this line if needed
    }

    return `<span class="status-indicator ${statusClass}" uk-tooltip="${statusText}" ${toggleAttribute}></span>${modalHtml}`;
  }

  /**
   * Create HTML markup for validator uptime window
   * @param {Object} status - Status data for a chain
   * @returns {string} HTML markup for uptime window
   * @private
   */
  _createUptimeWindow(status) {
    let window = `<div class="uk-width-1-2" style="text-align: end">`;

    if (status.missed === 0 && status.window === 0) {
      window += "error</div>";
    } else if (status.missed === 0) {
      window += `100%</div>`;
    } else {
      window += `${(100 - (status.missed / status.window) * 100).toFixed(2)}%</div>`;
    }

    window += `<div class="uk-width-1-2">${_.escape(status.missed)} / ${_.escape(status.window)}</div>`;

    return window;
  }

  /**
   * Create HTML markup for node status
   * @param {Object} status - Status data for a chain
   * @returns {string} HTML markup for node status
   * @private
   */
  _createNodeStatus(status) {
    let nodes = `${_.escape(status.healthy_nodes)} / ${_.escape(status.nodes)}`;

    if (status.healthy_nodes < status.nodes) {
      nodes =
        "<strong><span uk-icon='arrow-down' style='color: darkorange'></span>" +
        nodes +
        "</strong>";
    }

    return nodes;
  }

  /**
   * Determine CSS class for height animation
   * @param {string} chainId - Chain ID
   * @param {number} height - Block height
   * @returns {string} CSS animation class
   * @private
   */
  _getHeightAnimationClass(chainId, height) {
    // Check with grid renderer if available
    if (
      this.gridRenderer &&
      this.gridRenderer.updateBlockHeight(chainId, height)
    ) {
      return "block-height-change";
    } else {
      return "";
    }
  }

  /**
   * Update the status table with new data
   * @param {Object} status - The status data containing validator information
   */
  updateTable(status) {
    if (!status || !status.Status || !Array.isArray(status.Status)) {
      console.error("Invalid status data for table rendering");
      return;
    }

    // Clear the table
    while (this.statusTable.rows.length > 0) {
      this.statusTable.deleteRow(0);
    }

    // Render each status row
    for (let i = 0; i < status.Status.length; i++) {
      const chainStatus = status.Status[i];
      const row = this.statusTable.insertRow(i);
      let columnIndex = 0;

      // No longer add row-has-alert class
      // if (chainStatus.active_alerts > 0 || chainStatus.last_error !== '') {
      //  row.classList.add('row-has-alert');
      // }

      // Column: Status Indicator (potentially includes modal HTML)
      const statusHtml = this._createStatusIndicator(chainStatus);
      const statusCell = row.insertCell(columnIndex);
      statusCell.innerHTML = `<div style="text-align: center">${statusHtml}</div>`;
      statusCell.classList.add("status-cell");
      columnIndex++;

      // Column: Chain ID
      row.insertCell(columnIndex).innerHTML =
        `<div>${_.escape(chainStatus.name)} (${_.escape(chainStatus.chain_id)})</div>`;
      columnIndex++;

      // Column: Height with animation
      const heightClass = this._getHeightAnimationClass(
        chainStatus.chain_id,
        chainStatus.height,
      );
      const heightCell = row.insertCell(columnIndex);
      heightCell.innerHTML = `<div class="${heightClass}" data-chain="${chainStatus.chain_id}">${_.escape(chainStatus.height)}</div>`;
      heightCell.classList.add("height-data");
      columnIndex++;

      // Column: Moniker
      if (chainStatus.moniker === "not connected") {
        row.insertCell(columnIndex).innerHTML =
          `<div class="uk-text-warning">${_.escape(chainStatus.moniker)}</div>`;
      } else {
        row.insertCell(columnIndex).innerHTML =
          `<div>${_.escape(chainStatus.moniker)}</div>`;
      }
      columnIndex++;

      // Column: Voting Power
      row.insertCell(columnIndex).innerHTML =
        `<div class="uk-text-center"><span class="uk-width-1-2">${(100 * chainStatus.voting_power_percent).toFixed(1)}%</span></div>`;
      columnIndex++;

      // Column: Validator Projected 30-day Rewards
      row.insertCell(columnIndex).innerHTML =
        `<div class="uk-text-center"><span class="uk-width-1-2">${chainStatus.projected_30d_rewards.toFixed(1)}</span></div>`;
      columnIndex++;

      // Column: Unvoted Proposals
      row.insertCell(columnIndex).innerHTML =
        `<div style="text-align: center">${chainStatus.unvoted_open_gov_proposals}</div>`;
      columnIndex++;

      // Column: Uptime window
      const uptimeCell = row.insertCell(columnIndex);
      uptimeCell.innerHTML = `<div uk-grid>${this._createUptimeWindow(chainStatus)}</div>`;
      uptimeCell.classList.add("numeric-data");
      columnIndex++;

      // Column: Threshold
      const thresholdCell = row.insertCell(columnIndex);
      thresholdCell.innerHTML = `<div class="uk-text-center"><span class="uk-width-1-2">${100 * chainStatus.min_signed_per_window}%</span></div>`;
      thresholdCell.classList.add("numeric-data");
      columnIndex++;

      // Column: RPC Nodes
      const rpcCell = row.insertCell(columnIndex);
      rpcCell.innerHTML = `<div class="uk-text-center">${this._createNodeStatus(chainStatus)}</div>`;
      rpcCell.classList.add("numeric-data");
      columnIndex++;
    }
  }
}
