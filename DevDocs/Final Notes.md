# Final Notes & Production Readiness

Congratulations on completing the integration of Pezhvak Core v1.0. You now have a powerful, decentralized mesh engine driving your application. Before you move to production or distribute your app to users in high-risk environments, please review these final technical and safety considerations.

## 1. Security Disclaimer

While Pezhvak Core provides industry-standard End-to-End Encryption (E2EE) using NaCl `box`, no system is 100% "invisible."

*   **Traffic Analysis:** Even though `MessageID`s are hashed and payloads are encrypted, a sophisticated adversary with physical proximity can still see that Bluetooth packets are being exchanged.
*   **Metadata:** The `SenderId` and `RecipientId` are required for mesh routing. While they are not linked to real-world identities within the Core, their persistence in the "seen" logs for 72 hours is a trade-off for mesh reliability.
*   **Device Security:** The Core can only protect data at rest (via the Panic Wipe) and data in transit. It cannot protect against a compromised operating system (keyloggers or screen scrapers).

## 2. Hardware Limitations

Bluetooth Low Energy (BLE) performance varies wildly across hardware.

*   **Android Fragmentation:** Some budget Android devices have poor BLE stack implementations that may drop connections frequently. Always implement robust error handling in your `SendBLE` implementation.
*   **iOS Backgrounding:** iOS is very aggressive about suspending Bluetooth activity when an app is in the background. For a "Revolutionary" use case, encourage users to keep the app in the foreground during active data synchronization.

## 3. Pre-Deployment Checklist

- [ ] **Identity Persistence:** Ensure your app correctly saves and loads the `identity.json` file. If a user loses this file, they lose their cryptographic identity.
- [ ] **Database Lifecycle:** Verify that `pezhvak.close()` is called every time the app process is terminated to prevent BadgerDB lockfile issues.
- [ ] **Panic Button Visibility:** The "Panic Wipe" should be easily accessible but protected by a confirmation step to prevent accidental data loss.
- [ ] **Battery Monitoring:** If the user stays in **Max** power mode, provide a UI warning about increased battery consumption.
- [ ] **Field Testing:** Test the mesh relay logic with at least three devices. Verify that Device A can send a message to Device C through Device B.

## 4. Future Roadmap

Pezhvak Core is an evolving project. Future versions (v1.1+) are expected to include:

*   **Gossip Manifests:** Reducing redundant data transfer by allowing nodes to "handshake" their list of carried messages before sending full payloads.
*   **Binary Transparency:** Automated builds for the Core to ensure the `.aar` and `.xcframework` files match the open-source Go code.
*   **Local Encryption:** An option to encrypt the BadgerDB store itself with a user-provided passphrase.

## 5. Community and Support

If you encounter bugs in the Core logic or have suggestions for the `NativePlatform` interface, please contribute via the official repository. 

Stay safe, and keep the lines of communication open.

---

**Pezhvak Core Team**  
*Version 1.0.0 "Resilience"*