<script lang="ts">
	import { onMount } from 'svelte';
	import { Ws, type Message } from '#lib';

	// Data Interfaces
	interface UserProfile {
		id: string;
		username: string;
		avatar: string;
		online: boolean;
		status?: string;
	}

	interface SavedIdentity {
		username: string;
		avatar: string;
		status?: string;
		lastUsed: number;
	}

	interface RoomItem {
		id: string;
		name: string;
		topic: string;
		icon: string;
		creatorId: string;
		creatorName: string;
		isProtected: boolean;
		members: string[];
		pending: string[];
		createdAt: string;
	}

	interface PermissionRequest {
		roomId: string;
		roomName: string;
		requesterId: string;
		requesterName: string;
		requesterAvatar: string;
		time: string;
	}

	interface FileAttachment {
		id: string;
		name: string;
		size: number;
		formattedSize: string;
		type: string;
		dataUrl: string;
		isImage: boolean;
		isAudio: boolean;
		isVideo: boolean;
	}

	interface ReadReceipt {
		userId: string;
		username: string;
		avatar: string;
		readAt: string;
	}

	interface ChatMessage {
		id: string;
		sender: string;
		senderName?: string;
		senderAvatar?: string;
		room: string;
		text: string;
		files?: FileAttachment[];
		time: string;
		timestamp?: number;
		isSelf: boolean;
		recalled?: boolean;
		readBy?: ReadReceipt[];
	}

	interface DmMessage {
		id: string;
		fromId: string;
		fromName: string;
		fromAvatar: string;
		toId: string;
		text: string;
		files?: FileAttachment[];
		time: string;
		timestamp?: number;
		isSelf: boolean;
		recalled?: boolean;
		read?: boolean;
		readAt?: string;
	}

	interface WallPost {
		id: string;
		authorId: string;
		authorName: string;
		authorAvatar: string;
		content: string;
		time: string;
	}

	interface GameState {
		board: string[];
		turn: string;
		winner: string;
		scoreX: number;
		scoreO: number;
		lastBy: string;
	}

	interface LogEvent {
		id: string;
		type: 'in' | 'out' | 'sys' | 'err';
		time: string;
		summary: string;
		details?: any;
	}

	// Avatars for login
	const AVATARS = ['👨‍💻', '👩‍🚀', '🧙‍♂️', '🦊', '⚡', '🤖', '🐱', '🦄', '🥷', '👾'];

	// Reactive State (Svelte 5 Runes)
	let wsUrl = $state('ws://localhost:5000/ws');
	let status = $state<'connected' | 'connecting' | 'disconnected'>('disconnected');
	let myClientId = $state<string>('');
	let pingMs = $state<number | null>(null);

	// Auth / Login State
	let isLoggedIn = $state<boolean>(false);
	let usernameInput = $state<string>('');
	let userTitleInput = $state<string>('');
	let selectedAvatar = $state<string>('👨‍💻');
	let myProfile = $state<UserProfile | null>(null);
	let savedIdentities = $state<SavedIdentity[]>([]);
	let rememberIdentity = $state<boolean>(true);
	let autoLoginOnStartup = $state<boolean>(false);

	// Active Navigation Tab
	let activeTab = $state<'chat' | 'dm' | 'wall' | 'video' | 'game' | 'request' | 'logs'>('chat');

	// Connected Users
	let onlineUsers = $state<UserProfile[]>([]);
	let selectedDmUser = $state<UserProfile | null>(null);

	// Channels & Group Permission State
	let roomList = $state<RoomItem[]>([]);
	let currentRoom = $state<string>('general');
	let showCreateRoomModal = $state<boolean>(false);
	let newRoomForm = $state({
		name: '',
		topic: '',
		icon: '💬',
		isProtected: false
	});
	let showPermissionModal = $state<RoomItem | null>(null);
	let pendingPermissionRequests = $state<PermissionRequest[]>([]);
	let showOwnerRequestsModal = $state<boolean>(false);
	let joinRequestStatus = $state<Record<string, 'pending' | 'denied'>>({});
	let toastNotification = $state<{ text: string; type: 'info' | 'success' | 'warn' } | null>(null);

	let currentRoomData = $derived(roomList.find((r) => r.name === currentRoom || r.id === currentRoom));
	let chatInput = $state<string>('');
	let chatAttachedFiles = $state<FileAttachment[]>([]);
	let isDraggingChat = $state<boolean>(false);
	let chatFileInputElem = $state<HTMLInputElement | null>(null);
	let messages = $state<ChatMessage[]>([]);
	let currentRoomMessages = $derived(messages.filter((m) => m.room === currentRoom));
	let chatBoxElement = $state<HTMLDivElement | null>(null);

	// Direct Messages State
	let dmConversations = $state<Record<string, DmMessage[]>>({});
	let dmInput = $state<string>('');
	let dmAttachedFiles = $state<FileAttachment[]>([]);
	let isDraggingDm = $state<boolean>(false);
	let dmFileInputElem = $state<HTMLInputElement | null>(null);
	let dmBoxElement = $state<HTMLDivElement | null>(null);
	let unreadDms = $state<Record<string, number>>({});
	let totalUnreadDms = $derived(Object.values(unreadDms).reduce((a, b) => a + b, 0));
	let roomUnread = $state<Record<string, number>>({});
	let totalRoomUnread = $derived(Object.values(roomUnread).reduce((a, b) => a + b, 0));

	// Wall State
	let wallPosts = $state<WallPost[]>([]);
	let wallInput = $state<string>('');
	let showWallBanner = $state<boolean>(true);
	let quickWallInput = $state<string>('');

	// Photo & Video Call State
	let localVideoElem = $state<HTMLVideoElement | null>(null);
	let remoteVideoElem = $state<HTMLVideoElement | null>(null);
	let localMediaStream = $state<MediaStream | null>(null);
	let peerConnection = $state<RTCPeerConnection | null>(null);
	let isInCall = $state<boolean>(false);
	let activeCallPeer = $state<UserProfile | null>(null);
	let incomingCall = $state<{ fromId: string; fromName: string; fromAvatar: string; offer: any } | null>(null);
	let capturedPhoto = $state<string | null>(null);
	let receivedPhotos = $state<{ sender: string; avatar: string; dataUrl: string; time: string }[]>([]);

	// Real-Time Mini-Game State (Multiplayer Tic-Tac-Toe via Chaining)
	let game = $state<GameState>({
		board: Array(9).fill(''),
		turn: 'X',
		winner: '',
		scoreX: 0,
		scoreO: 0,
		lastBy: ''
	});
	let isAiThinking = $derived(game.turn === 'O' && !game.winner);

	// Request-Reply Console State
	let reqTitle = $state<string>('echo');
	let reqPayload = $state<string>('{"message": "Hello GinWS!"}');
	let reqTimeout = $state<number>(3000);
	let reqLoading = $state<boolean>(false);
	let reqResult = $state<{ status: 'success' | 'error'; latencyMs: number; data: any } | null>(null);

	// Diagnostics Logs
	let logs = $state<LogEvent[]>([]);
	let logFilter = $state<'all' | 'in' | 'out' | 'err'>('all');

	let wsInstance: Ws | null = null;

	function addLog(type: LogEvent['type'], summary: string, details?: any) {
		const entry: LogEvent = {
			id: Math.random().toString(36).substring(2, 9),
			type,
			time: new Date().toLocaleTimeString(),
			summary,
			details
		};
		logs = [entry, ...logs.slice(0, 199)];
	}

	// Connect WebSocket
	function connectWebSocket() {
		if (wsInstance) {
			wsInstance.closeSafe();
		}

		status = 'connecting';
		addLog('sys', `Connecting to ${wsUrl}...`);

		wsInstance = new Ws(wsUrl, {
			reconnectInterval: 2500,
			autoReconnect: true
		});

		wsInstance.on('open', () => {
			status = 'connected';
			addLog('sys', 'Connected to GinWS server');

			// Always query active channels & groups on connection
			wsInstance?.sendRaw('rooms:get');

			// Automatic persona login for instant demo experience
			if (myProfile) {
				performLogin(myProfile.username, myProfile.avatar, myProfile.status);
			} else {
				let loginName = 'Neo';
				let loginAvatar = '⚡';
				let loginStatus = '🚀 Real-Time Demo';
				try {
					if (typeof localStorage !== 'undefined') {
						const activeStored = localStorage.getItem(STORAGE_ACTIVE_IDENTITY_KEY);
						if (activeStored) {
							const active = JSON.parse(activeStored);
							if (active?.username) {
								loginName = active.username;
								loginAvatar = active.avatar || '⚡';
								loginStatus = active.status || '🚀 Real-Time Demo';
							}
						}
					}
				} catch (e) {
					console.warn('Auto-login error', e);
				}
				performLogin(loginName, loginAvatar, loginStatus);
			}

			// Query initial data
			wsInstance?.sendRaw('users:list');
			wsInstance?.sendRaw('wall:get');
			wsInstance?.sendRaw('game:get');
			wsInstance?.sendRaw('inbox:summary');

			// Join default room & fetch message history (offline catch-up)
			if (currentRoom) {
				wsInstance?.sendRaw('join', currentRoom, currentRoom);
				fetchRoomHistory(currentRoom);
			}
			measurePing();
		});

		wsInstance.on('close', () => {
			status = 'disconnected';
			addLog('sys', 'Disconnected from server');
		});

		wsInstance.on('error', (err: any) => {
			addLog('err', 'WebSocket error', err);
		});

		wsInstance.on('reconnecting', (info: any) => {
			status = 'connecting';
			addLog('sys', `Reconnecting (attempt #${info?.attempt || 1})...`);
		});

		wsInstance.on('reconnect', () => {
			status = 'connected';
			addLog('sys', 'Reconnected successfully');
			wsInstance?.sendRaw('rooms:get');
			wsInstance?.sendRaw('inbox:summary');
			if (myProfile) {
				performLogin(myProfile.username, myProfile.avatar, myProfile.status);
			}
			if (currentRoom) {
				wsInstance?.sendRaw('join', currentRoom, currentRoom);
				fetchRoomHistory(currentRoom);
			}
		});

		wsInstance.on('message', (msg: Message) => {
			addLog('in', `[${msg.title}] ${msg.room ? `#${msg.room}` : ''}`, msg);
		});

		// --- NAMED ACTION HANDLERS ---
		// 1. Room Chat Messages
		wsInstance.use('chat_message', (msg: Message) => {
			let stored = msg.payload;
			let msgId = msg.id || Math.random().toString(36).substring(2, 9);
			let senderId = (msg.data as string) || 'User';
			let senderName = '';
			let senderAvatar = '👤';
			let text = '';
			let files: FileAttachment[] = [];
			let time = new Date().toLocaleTimeString();
			let timestamp = Date.now();
			let recalled = false;
			let readBy: ReadReceipt[] = [];

			if (stored && typeof stored === 'object') {
				if (stored.id) msgId = stored.id;
				if (stored.senderId) senderId = stored.senderId;
				if (stored.senderName) senderName = stored.senderName;
				if (stored.senderAvatar) senderAvatar = stored.senderAvatar;
				if (stored.time) time = stored.time;
				if (stored.timestamp) timestamp = stored.timestamp;
				if (stored.recalled !== undefined) recalled = stored.recalled;
				if (Array.isArray(stored.readBy)) readBy = stored.readBy;

				let content = stored.content !== undefined ? stored.content : stored;
				if (typeof content === 'string') {
					text = content;
				} else if (content && typeof content === 'object') {
					text = content.text || '';
					if (Array.isArray(content.files)) files = content.files;
				}
			} else if (typeof stored === 'string') {
				text = stored;
			}

			if (!senderName) {
				const senderProfile = onlineUsers.find((u) => u.id === senderId);
				senderName = senderProfile ? senderProfile.username : senderId;
				senderAvatar = senderProfile ? senderProfile.avatar : senderAvatar;
			}

			const isSelf = senderId === myClientId;
			const roomName = msg.room || currentRoom || 'general';

			const newMsg: ChatMessage = {
				id: msgId,
				sender: senderId,
				senderName,
				senderAvatar,
				room: roomName,
				text,
				files,
				time,
				timestamp,
				isSelf,
				recalled,
				readBy
			};

			const idx = messages.findIndex((m) => m.id === msgId);
			if (idx >= 0) {
				messages[idx] = newMsg;
			} else {
				messages = [...messages, newMsg];
			}
			saveMessagesBackup(messages);

			// If viewing current room and received message from peer, send read receipt
			if (isLoggedIn && roomName === currentRoom && !isSelf && wsInstance && status === 'connected') {
				markMessageAsRead(msgId, roomName);
				roomUnread[roomName] = 0;
			} else if (roomName !== currentRoom && !isSelf) {
				roomUnread[roomName] = (roomUnread[roomName] || 0) + 1;
			}

			setTimeout(scrollChatToBottom, 40);
			return '';
		});

		// 2. Historical Room Messages (Offline Catch-Up & Reconnect Sync)
		wsInstance.use('room:messages:history', (msg: Message) => {
			const payload = msg.payload;
			if (!payload || !Array.isArray(payload.messages)) return '';
			const roomName = payload.room || msg.room;

			const newItems: ChatMessage[] = [];
			for (const sm of payload.messages) {
				let text = '';
				let files: FileAttachment[] = [];
				if (typeof sm.content === 'string') {
					text = sm.content;
				} else if (sm.content && typeof sm.content === 'object') {
					text = sm.content.text || '';
					if (Array.isArray(sm.content.files)) files = sm.content.files;
				}

				const isSelf = sm.senderId === myClientId;
				newItems.push({
					id: sm.id,
					sender: sm.senderId,
					senderName: sm.senderName || sm.senderId,
					senderAvatar: sm.senderAvatar || '👤',
					room: sm.room || roomName,
					text,
					files,
					time: sm.time || '',
					timestamp: sm.timestamp || 0,
					isSelf,
					recalled: !!sm.recalled,
					readBy: Array.isArray(sm.readBy) ? sm.readBy : []
				});

				// Automatically mark message as read if not seen yet
				if (
					isLoggedIn &&
					roomName === currentRoom &&
					!isSelf &&
					(!sm.readBy || !sm.readBy.some((r: any) => r.userId === myClientId))
				) {
					markMessageAsRead(sm.id, roomName);
				}
			}

			// Merge with existing messages, deduplicating by ID
			const existingOthers = messages.filter((m) => m.room !== roomName);
			const currentRoomExisting = messages.filter((m) => m.room === roomName);
			const idMap = new Map<string, ChatMessage>();

			for (const m of newItems) {
				idMap.set(m.id, m);
			}
			for (const m of currentRoomExisting) {
				if (!idMap.has(m.id)) {
					idMap.set(m.id, m);
				}
			}

			const mergedCurrent = Array.from(idMap.values()).sort(
				(a, b) => (a.timestamp || 0) - (b.timestamp || 0)
			);
			messages = [...existingOthers, ...mergedCurrent];
			saveMessagesBackup(messages);
			setTimeout(scrollChatToBottom, 40);
			return '';
		});

		// 3. Message Read Receipts (Facebook-Style Seen Indicators)
		wsInstance.use('message:read_receipt', (msg: Message) => {
			const data = msg.payload;
			if (!data || !data.messageId) return '';
			const msgId = data.messageId;
			const updatedReadBy = data.readBy;
			const receipt = data.receipt;

			const idx = messages.findIndex((m) => m.id === msgId);
			if (idx >= 0) {
				if (Array.isArray(updatedReadBy)) {
					messages[idx].readBy = updatedReadBy;
				} else if (receipt) {
					const cur = messages[idx].readBy || [];
					if (!cur.some((r) => r.userId === receipt.userId)) {
						messages[idx].readBy = [...cur, receipt];
					}
				}
				messages = [...messages]; // trigger reactivity
				saveMessagesBackup(messages);
			}
			return '';
		});

		// 4. Message Recalled (Unsend)
		wsInstance.use('message:recalled', (msg: Message) => {
			const data = msg.payload;
			if (!data || !data.messageId) return '';
			const msgId = data.messageId;

			const idx = messages.findIndex((m) => m.id === msgId);
			if (idx >= 0) {
				messages[idx].recalled = true;
				messages[idx].text = '🚫 This message was recalled';
				messages[idx].files = [];
				messages = [...messages]; // trigger reactivity
				saveMessagesBackup(messages);
			}
			return '';
		});

		// 5. Online Users Updates
		wsInstance.use('users:updated', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				onlineUsers = msg.payload;
			}
			return '';
		});

		wsInstance.use('users:list', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				onlineUsers = msg.payload;
			}
			return '';
		});

		// 6. Direct Messages
		wsInstance.use('dm_message', (msg: Message) => {
			const data = msg.payload;
			if (!data) return '';
			const partnerId = data.fromId;

			let text = '';
			let files: FileAttachment[] = [];

			if (typeof data.content === 'string') {
				text = data.content;
			} else if (data.content && typeof data.content === 'object') {
				text = data.content.text || '';
				if (Array.isArray(data.content.files)) {
					files = data.content.files;
				}
			}

			const msgId = data.id || Math.random().toString(36).substring(2, 9);
			const newDm: DmMessage = {
				id: msgId,
				fromId: data.fromId,
				fromName: data.fromName || data.fromId,
				fromAvatar: data.fromAvatar || '👤',
				toId: data.toId,
				text,
				files,
				time: data.time || new Date().toLocaleTimeString(),
				timestamp: data.timestamp || Date.now(),
				isSelf: false,
				read: !!data.read,
				readAt: data.readAt
			};

			const list = dmConversations[partnerId] || [];
			dmConversations = { ...dmConversations, [partnerId]: [...list, newDm] };

			// If currently viewing DM with this partner, send read acknowledgment
			if (activeTab === 'dm' && selectedDmUser?.id === partnerId && wsInstance && status === 'connected') {
				wsInstance.sendRaw('dm:read', partnerId, { partnerId, messageId: msgId });
			} else {
				unreadDms[partnerId] = (unreadDms[partnerId] || 0) + 1;
				showToast(`💬 New DM from ${data.fromName || 'Peer'}: "${text.slice(0, 32)}${text.length > 32 ? '...' : ''}"`, 'info');
			}

			setTimeout(scrollDmToBottom, 40);
			return '';
		});

		// 7. DM History Catch-Up
		wsInstance.use('dm:messages:history', (msg: Message) => {
			const payload = msg.payload;
			if (!payload || !payload.partnerId || !Array.isArray(payload.messages)) return '';
			const partnerId = payload.partnerId;

			const list: DmMessage[] = payload.messages.map((dm: any) => {
				let text = '';
				let files: FileAttachment[] = [];
				if (typeof dm.content === 'string') {
					text = dm.content;
				} else if (dm.content && typeof dm.content === 'object') {
					text = dm.content.text || '';
					if (Array.isArray(dm.content.files)) files = dm.content.files;
				}
				const isSelf = dm.fromId === myClientId;
				return {
					id: dm.id,
					fromId: dm.fromId,
					fromName: dm.fromName,
					fromAvatar: dm.fromAvatar,
					toId: dm.toId,
					text,
					files,
					time: dm.time || '',
					timestamp: dm.timestamp || 0,
					isSelf,
					recalled: !!dm.recalled,
					read: !!dm.read,
					readAt: dm.readAt
				};
			});

			dmConversations = { ...dmConversations, [partnerId]: list };
			setTimeout(scrollDmToBottom, 40);
			return '';
		});

		// 8. DM Read Receipt
		wsInstance.use('dm:read_receipt', (msg: Message) => {
			const data = msg.payload;
			if (!data || !data.messageId) return '';
			const msgId = data.messageId;

			for (const partnerId of Object.keys(dmConversations)) {
				const conv = dmConversations[partnerId];
				const idx = conv.findIndex((m) => m.id === msgId);
				if (idx >= 0) {
					conv[idx].read = true;
					conv[idx].readAt = data.readAt;
					dmConversations = { ...dmConversations, [partnerId]: [...conv] };
					break;
				}
			}
			return '';
		});

		// 4. Public Wall Updates
		wsInstance.use('wall:new_post', (msg: Message) => {
			const post = msg.payload;
			if (post) {
				wallPosts = [
					{
						id: post.id || Math.random().toString(),
						authorId: post.authorId || '',
						authorName: post.authorName || 'Anonymous',
						authorAvatar: post.authorAvatar || '📢',
						content: post.content || post.text || '',
						time: post.time || new Date().toLocaleTimeString()
					},
					...wallPosts
				];
			}
			return '';
		});

		wsInstance.use('wall:posts', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				wallPosts = msg.payload.map((p) => ({
					id: p.id || Math.random().toString(),
					authorId: p.authorId || '',
					authorName: p.authorName || 'Anonymous',
					authorAvatar: p.authorAvatar || '📢',
					content: p.content || p.text || '',
					time: p.time || ''
				}));
			}
			return '';
		});

		// 5. WebRTC Signaling & Photo Call
		wsInstance.use('signal:incoming_call', async (msg: Message) => {
			const callerId = msg.data as string;
			const offer = msg.payload?.offer;
			const callerProfile = onlineUsers.find((u) => u.id === callerId);

			incomingCall = {
				fromId: callerId,
				fromName: callerProfile ? callerProfile.username : callerId,
				fromAvatar: callerProfile ? callerProfile.avatar : '📞',
				offer
			};
			return '';
		});

		wsInstance.use('signal:call_answered', async (msg: Message) => {
			if (peerConnection && msg.payload?.answer) {
				await peerConnection.setRemoteDescription(new RTCSessionDescription(msg.payload.answer));
				addLog('sys', 'Peer answered call. Media stream connected!');
			}
			return '';
		});

		wsInstance.use('signal:ice_candidate', async (msg: Message) => {
			if (peerConnection && msg.payload?.candidate) {
				try {
					await peerConnection.addIceCandidate(new RTCIceCandidate(msg.payload.candidate));
				} catch (e) {
					console.error('Error adding ICE candidate', e);
				}
			}
			return '';
		});

		wsInstance.use('signal:call_ended', () => {
			endCall(false);
			addLog('sys', 'Call ended by remote peer');
			return '';
		});

		wsInstance.use('photo:received', (msg: Message) => {
			const senderId = (msg.data as string) || msg.id || 'User';
			const senderProfile = onlineUsers.find((u) => u.id === senderId);
			receivedPhotos = [
				{
					sender: senderProfile ? senderProfile.username : senderId,
					avatar: senderProfile ? senderProfile.avatar : '📸',
					dataUrl: msg.payload?.dataUrl || msg.payload,
					time: new Date().toLocaleTimeString()
				},
				...receivedPhotos
			];
			return '';
		});

		// 6. Mini-Game State Synchronization
		wsInstance.use('game:updated', (msg: Message) => {
			if (msg.payload) {
				game = {
					board: Array.isArray(msg.payload.board) ? msg.payload.board : Array(9).fill(''),
					turn: msg.payload.turn || 'X',
					winner: msg.payload.winner || '',
					scoreX: msg.payload.scoreX || 0,
					scoreO: msg.payload.scoreO || 0,
					lastBy: msg.payload.lastBy || ''
				};
			}
			return '';
		});

		// 7. Rooms & Groups Action Handlers
		wsInstance.use('rooms:list', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				roomList = msg.payload;
			}
			return '';
		});

		wsInstance.use('rooms:updated', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				roomList = msg.payload;
			}
			return '';
		});

		wsInstance.use('room:permission_request', (msg: Message) => {
			const req = msg.payload as PermissionRequest;
			if (req) {
				pendingPermissionRequests = [
					req,
					...pendingPermissionRequests.filter(
						(r) => !(r.roomId === req.roomId && r.requesterId === req.requesterId)
					)
				];
				showToast(`🔔 ${req.requesterName} requested permission to join group #${req.roomName}`, 'warn');
				addLog('sys', `Permission request from ${req.requesterName} for group #${req.roomName}`, req);
			}
			return '';
		});

		wsInstance.use('room:access_granted', (msg: Message) => {
			const data = msg.payload;
			if (data?.roomId) {
				delete joinRequestStatus[data.roomId];
				showToast(`🎉 Access Granted to #${data.roomName || data.roomId}!`, 'success');
				addLog('sys', `Access granted to group #${data.roomName || data.roomId}`);
				selectRoom(data.roomId);
				if (showPermissionModal?.id === data.roomId) {
					showPermissionModal = null;
				}
			}
			return '';
		});

		wsInstance.use('room:access_denied', (msg: Message) => {
			const data = msg.payload;
			if (data?.roomId) {
				joinRequestStatus[data.roomId] = 'denied';
				showToast(`❌ Access declined to group #${data.roomName || data.roomId}`, 'warn');
				addLog('sys', `Access denied to group #${data.roomName || data.roomId}`);
			}
			return '';
		});

		wsInstance.use('room:request_pending', (msg: Message) => {
			const data = msg.payload;
			if (data?.roomId) {
				joinRequestStatus[data.roomId] = 'pending';
				showToast(`⏳ Join request sent to group owner. Awaiting approval...`, 'info');
			}
			return '';
		});

		// 9. Inbox Summary (Unread Badges across Channels & DMs)
		wsInstance.use('inbox:summary', (msg: Message) => {
			if (Array.isArray(msg.payload)) {
				for (const item of msg.payload) {
					if (item.type === 'room') {
						if (item.id !== currentRoom) {
							roomUnread[item.id] = item.unreadCount || 0;
						}
					} else if (item.type === 'dm') {
						if (!activeDmUser || activeDmUser.id !== item.id) {
							unreadDms[item.id] = item.unreadCount || 0;
						}
					}
				}
				addLog('sys', 'Inbox summary synced (badges updated)', msg.payload);
			}
			return '';
		});
	}

	function showToast(text: string, type: 'info' | 'success' | 'warn' = 'info') {
		toastNotification = { text, type };
		setTimeout(() => {
			if (toastNotification?.text === text) {
				toastNotification = null;
			}
		}, 4000);
	}

	// Storage Keys for Browser Identity Persistence
	const STORAGE_IDENTITIES_KEY = 'ginws_saved_identities';
	const STORAGE_ACTIVE_IDENTITY_KEY = 'ginws_active_identity';
	const STORAGE_AUTO_LOGIN_KEY = 'ginws_auto_login';

	function loadSavedIdentities() {
		try {
			if (typeof localStorage === 'undefined') return;
			const autoLoginStored = localStorage.getItem(STORAGE_AUTO_LOGIN_KEY);
			if (autoLoginStored) {
				autoLoginOnStartup = JSON.parse(autoLoginStored);
			}
			const stored = localStorage.getItem(STORAGE_IDENTITIES_KEY);
			if (stored) {
				const parsed = JSON.parse(stored);
				if (Array.isArray(parsed)) {
					savedIdentities = parsed;
				}
			}
			const activeStored = localStorage.getItem(STORAGE_ACTIVE_IDENTITY_KEY);
			if (activeStored) {
				const active = JSON.parse(activeStored);
				if (active?.username) {
					usernameInput = active.username;
					selectedAvatar = active.avatar || '👨‍💻';
					userTitleInput = active.status || '';
				}
			} else if (savedIdentities.length > 0) {
				usernameInput = savedIdentities[0].username;
				selectedAvatar = savedIdentities[0].avatar || '👨‍💻';
				userTitleInput = savedIdentities[0].status || '';
			}
		} catch (e) {
			console.warn('Failed to load identities from localStorage', e);
		}
	}

	function saveIdentityToStorage(username: string, avatar: string, statusText?: string) {
		try {
			if (typeof localStorage === 'undefined') return;
			const now = Date.now();
			const existingIndex = savedIdentities.findIndex((i) => i.username.toLowerCase() === username.toLowerCase());
			if (existingIndex >= 0) {
				const updated = [...savedIdentities];
				updated[existingIndex] = {
					username,
					avatar,
					status: statusText || updated[existingIndex].status || '',
					lastUsed: now
				};
				const item = updated.splice(existingIndex, 1)[0];
				savedIdentities = [item, ...updated];
			} else {
				savedIdentities = [
					{ username, avatar, status: statusText || '', lastUsed: now },
					...savedIdentities
				];
			}
			savedIdentities = savedIdentities.slice(0, 10);
			localStorage.setItem(STORAGE_IDENTITIES_KEY, JSON.stringify(savedIdentities));
			localStorage.setItem(STORAGE_ACTIVE_IDENTITY_KEY, JSON.stringify({ username, avatar, status: statusText }));
			localStorage.setItem(STORAGE_AUTO_LOGIN_KEY, JSON.stringify(autoLoginOnStartup));
		} catch (e) {
			console.warn('Failed to save identity to localStorage', e);
		}
	}

	function removeSavedIdentity(username: string) {
		savedIdentities = savedIdentities.filter((i) => i.username.toLowerCase() !== username.toLowerCase());
		try {
			if (typeof localStorage !== 'undefined') {
				localStorage.setItem(STORAGE_IDENTITIES_KEY, JSON.stringify(savedIdentities));
				const activeStored = localStorage.getItem(STORAGE_ACTIVE_IDENTITY_KEY);
				if (activeStored) {
					const active = JSON.parse(activeStored);
					if (active?.username?.toLowerCase() === username.toLowerCase()) {
						localStorage.removeItem(STORAGE_ACTIVE_IDENTITY_KEY);
					}
				}
			}
		} catch (e) {
			console.warn('Failed to remove saved identity', e);
		}
	}

	function logoutAndSwitchIdentity() {
		isLoggedIn = false;
		try {
			if (typeof localStorage !== 'undefined') {
				localStorage.removeItem(STORAGE_ACTIVE_IDENTITY_KEY);
			}
		} catch {}
	}

	function selectSavedIdentity(identity: SavedIdentity) {
		usernameInput = identity.username;
		selectedAvatar = identity.avatar;
		userTitleInput = identity.status || '';
		performLogin(identity.username, identity.avatar, identity.status);
	}

	function formatLastUsed(timeMs: number): string {
		if (!timeMs) return '';
		const diff = Date.now() - timeMs;
		const minutes = Math.floor(diff / 60000);
		if (minutes < 1) return 'Just now';
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		return `${days}d ago`;
	}

	// Login Handler
	async function performLogin(username: string, avatar: string, statusText?: string) {
		if (!wsInstance || status !== 'connected') return;

		try {
			const res = await wsInstance.request('auth:login', { username, avatar, status: statusText }, { timeout: 4000 });
			if (res.payload) {
				myProfile = { ...res.payload, status: statusText };
				myClientId = res.payload.id;
				isLoggedIn = true;
				addLog('sys', `Logged in as ${username} (${myClientId})`);

				if (rememberIdentity) {
					saveIdentityToStorage(username, avatar, statusText);
				}

				// Fetch initial rooms, wall & game, and current room history
				wsInstance.sendRaw('rooms:get');
				wsInstance.sendRaw('wall:get');
				wsInstance.sendRaw('game:get');
				fetchRoomHistory(currentRoom);
			}
		} catch (err: any) {
			addLog('err', 'Login request failed', err);
		}
	}

	function handleLoginSubmit() {
		const name = usernameInput.trim();
		if (!name) return;
		performLogin(name, selectedAvatar, userTitleInput.trim());
	}

	// Channels & Group Permission Operations
	function selectRoom(roomName: string) {
		const targetRoom = roomList.find((r) => r.name === roomName || r.id === roomName);

		// If protected, check if authorized (owner or member)
		if (targetRoom && targetRoom.isProtected) {
			const isOwner = targetRoom.creatorId === myClientId || targetRoom.creatorId === 'system';
			const isMember = targetRoom.members && targetRoom.members.includes(myClientId);

			if (!isOwner && !isMember) {
				// User needs permission!
				showPermissionModal = targetRoom;
				return;
			}
		}

		if (currentRoom === roomName && wsInstance) {
			roomUnread[roomName] = 0;
			fetchRoomHistory(roomName);
			return;
		}
		if (currentRoom && wsInstance) {
			wsInstance.sendRaw('leave', currentRoom, currentRoom);
		}
		currentRoom = roomName;
		roomUnread[roomName] = 0;
		if (wsInstance) {
			wsInstance.sendRaw('join', currentRoom, currentRoom);
			fetchRoomHistory(currentRoom);
		}
	}

	function requestGroupPermission(room: RoomItem) {
		if (!wsInstance || status !== 'connected') return;
		wsInstance.sendRaw('room:request_access', null, { roomId: room.id });
		joinRequestStatus[room.id] = 'pending';
	}

	function answerPermissionRequest(req: PermissionRequest, approved: boolean) {
		if (!wsInstance || status !== 'connected') return;
		wsInstance.sendRaw('room:approve_access', null, {
			roomId: req.roomId,
			requesterId: req.requesterId,
			approved
		});
		pendingPermissionRequests = pendingPermissionRequests.filter(
			(r) => !(r.roomId === req.roomId && r.requesterId === req.requesterId)
		);
		showToast(
			approved ? `Approved ${req.requesterName} into #${req.roomName}` : `Declined ${req.requesterName}`,
			approved ? 'success' : 'warn'
		);
	}

	function submitCreateRoom() {
		const clean = newRoomForm.name.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
		if (!clean || !wsInstance || status !== 'connected') return;

		wsInstance.sendRaw('room:create', null, {
			name: clean,
			topic: newRoomForm.topic.trim(),
			icon: newRoomForm.icon || '💬',
			isProtected: newRoomForm.isProtected
		});

		showToast(
			newRoomForm.isProtected
				? `Created private group #${clean} (Permission Required)!`
				: `Created channel #${clean}!`,
			'success'
		);
		showCreateRoomModal = false;
		newRoomForm = {
			name: '',
			topic: '',
			icon: '💬',
			isProtected: false
		};
		setTimeout(() => selectRoom(clean), 200);
	}

	// File Handling & Drag-Drop Helpers
	function formatBytes(bytes: number, decimals = 1): string {
		if (bytes === 0) return '0 B';
		const k = 1024;
		const dm = decimals < 0 ? 0 : decimals;
		const sizes = ['B', 'KB', 'MB', 'GB'];
		const i = Math.floor(Math.log(bytes) / Math.log(k));
		return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
	}

	function readFileAsDataUrl(file: File): Promise<string> {
		return new Promise((resolve, reject) => {
			const reader = new FileReader();
			reader.onload = () => resolve(reader.result as string);
			reader.onerror = (e) => reject(e);
			reader.readAsDataURL(file);
		});
	}

	async function processFileList(fileList: FileList | File[]): Promise<FileAttachment[]> {
		const result: FileAttachment[] = [];
		const files = Array.from(fileList);

		for (const file of files) {
			if (file.size > 12 * 1024 * 1024) {
				addLog('err', `File "${file.name}" exceeds 12MB limit. Skipped.`);
				continue;
			}

			try {
				const dataUrl = await readFileAsDataUrl(file);
				const isImage = file.type.startsWith('image/');
				const isAudio = file.type.startsWith('audio/');
				const isVideo = file.type.startsWith('video/');

				result.push({
					id: Math.random().toString(36).substring(2, 9),
					name: file.name,
					size: file.size,
					formattedSize: formatBytes(file.size),
					type: file.type || 'application/octet-stream',
					dataUrl,
					isImage,
					isAudio,
					isVideo
				});
			} catch (err) {
				addLog('err', `Failed to read file "${file.name}"`, err);
			}
		}
		return result;
	}

	// Chat Drag & Drop / File Selection Handlers
	async function handleChatDrop(event: DragEvent) {
		event.preventDefault();
		isDraggingChat = false;
		if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
			const newFiles = await processFileList(event.dataTransfer.files);
			chatAttachedFiles = [...chatAttachedFiles, ...newFiles];
		}
	}

	function handleChatDragOver(event: DragEvent) {
		event.preventDefault();
		if (event.dataTransfer) {
			event.dataTransfer.dropEffect = 'copy';
		}
		isDraggingChat = true;
	}

	function handleChatDragLeave(event: DragEvent) {
		event.preventDefault();
		const currentTarget = event.currentTarget as HTMLElement | null;
		const relatedTarget = event.relatedTarget as HTMLElement | null;
		if (!currentTarget || !relatedTarget || !currentTarget.contains(relatedTarget)) {
			isDraggingChat = false;
		}
	}

	async function handleChatFileInput(event: Event) {
		const target = event.target as HTMLInputElement;
		if (target.files && target.files.length > 0) {
			const newFiles = await processFileList(target.files);
			chatAttachedFiles = [...chatAttachedFiles, ...newFiles];
			target.value = '';
		}
	}

	function removeChatAttachment(id: string) {
		chatAttachedFiles = chatAttachedFiles.filter((f) => f.id !== id);
	}

	function sendChatMessage() {
		const text = chatInput.trim();
		const hasFiles = chatAttachedFiles.length > 0;
		if ((!text && !hasFiles) || !wsInstance || status !== 'connected') return;

		const files = [...chatAttachedFiles];
		wsInstance.sendRaw('chat', currentRoom, { text, files, timestamp: Date.now() });
		chatInput = '';
		chatAttachedFiles = [];
	}

	function scrollChatToBottom() {
		if (chatBoxElement) {
			chatBoxElement.scrollTop = chatBoxElement.scrollHeight;
		}
	}

	function fetchRoomHistory(roomName: string) {
		if (!wsInstance || status !== 'connected') return;
		wsInstance.sendRaw('room:messages:get', roomName, { room: roomName });
	}

	function markMessageAsRead(messageId: string, roomName: string) {
		if (!wsInstance || status !== 'connected' || !myProfile) return;
		wsInstance.sendRaw('message:read', roomName, {
			room: roomName,
			messageId,
			userId: myClientId,
			username: myProfile.username,
			avatar: myProfile.avatar
		});
	}

	function recallMessage(messageId: string) {
		if (!wsInstance || status !== 'connected') return;
		wsInstance.sendRaw('message:recall', currentRoom, {
			room: currentRoom,
			messageId
		});
		showToast('Message recalled for everyone', 'info');
	}

	function loadCachedMessagesBackup() {
		try {
			if (typeof localStorage !== 'undefined') {
				const cached = localStorage.getItem('ginws_messages_cache');
				if (cached) {
					const parsed = JSON.parse(cached);
					if (Array.isArray(parsed) && parsed.length > 0) {
						messages = parsed;
					}
				}
			}
		} catch (e) {
			console.warn('Failed to load message backup', e);
		}
	}

	function saveMessagesBackup(items: ChatMessage[]) {
		try {
			if (typeof localStorage !== 'undefined') {
				localStorage.setItem('ginws_messages_cache', JSON.stringify(items.slice(-100)));
			}
		} catch {}
	}

	let isSimulatingOffline = $state<boolean>(false);
	function simulateOfflineRecovery() {
		if (isSimulatingOffline) return;
		isSimulatingOffline = true;
		showToast('🔌 Disconnected! Simulating 2.5s network drop...', 'warn');
		if (wsInstance) {
			wsInstance.closeSafe();
		}
		status = 'disconnected';

		setTimeout(() => {
			showToast('⚡ Reconnecting to GinWS server...', 'info');
			connectWebSocket();
			setTimeout(() => {
				isSimulatingOffline = false;
				showToast('✅ Reconnected! 100% of messages automatically recalled!', 'success');
			}, 800);
		}, 2500);
	}

	// Direct Messaging Operations
	function openDmWith(user: UserProfile) {
		selectedDmUser = user;
		unreadDms[user.id] = 0;
		activeTab = 'dm';
		if (wsInstance && status === 'connected') {
			wsInstance.sendRaw('dm:messages:get', user.id, { partnerId: user.id });
		}
	}

	// DM Drag & Drop / File Selection Handlers
	async function handleDmDrop(event: DragEvent) {
		event.preventDefault();
		isDraggingDm = false;
		if (event.dataTransfer?.files && event.dataTransfer.files.length > 0) {
			const newFiles = await processFileList(event.dataTransfer.files);
			dmAttachedFiles = [...dmAttachedFiles, ...newFiles];
		}
	}

	function handleDmDragOver(event: DragEvent) {
		event.preventDefault();
		if (event.dataTransfer) {
			event.dataTransfer.dropEffect = 'copy';
		}
		isDraggingDm = true;
	}

	function handleDmDragLeave(event: DragEvent) {
		event.preventDefault();
		const currentTarget = event.currentTarget as HTMLElement | null;
		const relatedTarget = event.relatedTarget as HTMLElement | null;
		if (!currentTarget || !relatedTarget || !currentTarget.contains(relatedTarget)) {
			isDraggingDm = false;
		}
	}

	async function handleDmFileInput(event: Event) {
		const target = event.target as HTMLInputElement;
		if (target.files && target.files.length > 0) {
			const newFiles = await processFileList(target.files);
			dmAttachedFiles = [...dmAttachedFiles, ...newFiles];
			target.value = '';
		}
	}

	function removeDmAttachment(id: string) {
		dmAttachedFiles = dmAttachedFiles.filter((f) => f.id !== id);
	}

	function sendDm() {
		const text = dmInput.trim();
		const hasFiles = dmAttachedFiles.length > 0;
		if ((!text && !hasFiles) || !selectedDmUser || !wsInstance || status !== 'connected') return;

		const targetId = selectedDmUser.id;
		const files = [...dmAttachedFiles];

		// Record locally for sender
		const newDm: DmMessage = {
			id: Math.random().toString(36).substring(2, 9),
			fromId: myClientId,
			fromName: myProfile?.username || 'You',
			fromAvatar: myProfile?.avatar || '👤',
			toId: targetId,
			text,
			files,
			time: new Date().toLocaleTimeString(),
			isSelf: true
		};

		const list = dmConversations[targetId] || [];
		dmConversations = { ...dmConversations, [targetId]: [...list, newDm] };

		// Send over socket with target id and payload
		const msg: Message = {
			title: 'dm',
			id: targetId,
			payload: {
				text,
				files
			}
		};
		wsInstance.sendObj(msg);

		dmInput = '';
		dmAttachedFiles = [];
		setTimeout(scrollDmToBottom, 40);
	}

	function scrollDmToBottom() {
		if (dmBoxElement) {
			dmBoxElement.scrollTop = dmBoxElement.scrollHeight;
		}
	}

	// Public Wall Operations
	function submitWallPost() {
		const content = wallInput.trim();
		if (!content || !wsInstance || status !== 'connected') return;

		wsInstance.sendRaw('wall:post', null, { content });
		wallInput = '';
		showToast('📢 Broadcast shout sent to Public Wall!', 'success');
	}

	function submitQuickWallPost() {
		const content = quickWallInput.trim();
		if (!content || !wsInstance || status !== 'connected') return;

		wsInstance.sendRaw('wall:post', null, { content });
		quickWallInput = '';
		showToast('📢 Broadcast shout sent to Public Wall!', 'success');
	}

	// Interactive Simulated MediaStream Generator (Webcam Fallback & Demo Calls)
	function createSimulatedMediaStream(avatar: string, label: string): MediaStream {
		if (typeof document === 'undefined') {
			return new MediaStream();
		}
		const canvas = document.createElement('canvas');
		canvas.width = 640;
		canvas.height = 480;
		const ctx = canvas.getContext('2d');
		if (!ctx) return new MediaStream();

		let frame = 0;
		let animId: number;

		const draw = () => {
			frame++;
			// Gradient background
			const grad = ctx.createLinearGradient(0, 0, 640, 480);
			grad.addColorStop(0, '#090d16');
			grad.addColorStop(0.5, '#161e2e');
			grad.addColorStop(1, '#1f2937');
			ctx.fillStyle = grad;
			ctx.fillRect(0, 0, 640, 480);

			// Pulsing ring
			const pulse = Math.sin(frame / 18) * 15;
			ctx.strokeStyle = '#38bdf8';
			ctx.lineWidth = 3;
			ctx.beginPath();
			ctx.arc(320, 200, 75 + pulse, 0, Math.PI * 2);
			ctx.stroke();

			// Avatar
			ctx.font = '72px system-ui, sans-serif';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';
			ctx.fillText(avatar, 320, 200);

			// Label
			ctx.font = 'bold 22px system-ui, sans-serif';
			ctx.fillStyle = '#f8fafc';
			ctx.fillText(label, 320, 310);

			// Live Status
			ctx.font = '13px monospace';
			ctx.fillStyle = '#4ade80';
			ctx.fillText(`🟢 LIVE STREAM ● ${new Date().toLocaleTimeString()} (720p 30fps)`, 320, 345);

			// Audio waveform bars
			for (let i = 0; i < 24; i++) {
				const barH = Math.abs(Math.sin((frame + i * 12) / 8)) * 30 + 4;
				ctx.fillStyle = i % 2 === 0 ? '#38bdf8' : '#818cf8';
				ctx.fillRect(170 + i * 13, 420 - barH, 9, barH);
			}

			animId = requestAnimationFrame(draw);
		};
		draw();

		const stream = (canvas as any).captureStream ? (canvas as any).captureStream(30) : new MediaStream();
		if (stream.getVideoTracks().length > 0) {
			stream.getVideoTracks()[0].onended = () => cancelAnimationFrame(animId);
		}
		return stream;
	}

	// Camera & Video Call (WebRTC & Photo Snapshot)
	async function initCamera() {
		if (localMediaStream) return;
		try {
			if (typeof navigator !== 'undefined' && navigator.mediaDevices?.getUserMedia) {
				localMediaStream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
				if (localVideoElem) {
					localVideoElem.srcObject = localMediaStream;
				}
				addLog('sys', 'Webcam stream active');
			} else {
				throw new Error('getUserMedia not available');
			}
		} catch (err) {
			console.warn('Physical camera unavailable or permission denied; switching to simulated test video feed', err);
			localMediaStream = createSimulatedMediaStream(myProfile?.avatar || '⚡', myProfile?.username || 'You');
			if (localVideoElem) {
				localVideoElem.srcObject = localMediaStream;
			}
			addLog('sys', 'Using simulated interactive video feed');
		}
	}

	function snapPhoto() {
		if (!localVideoElem) return;
		const canvas = document.createElement('canvas');
		canvas.width = localVideoElem.videoWidth || 640;
		canvas.height = localVideoElem.videoHeight || 480;
		const ctx = canvas.getContext('2d');
		if (ctx) {
			ctx.drawImage(localVideoElem, 0, 0, canvas.width, canvas.height);
			capturedPhoto = canvas.toDataURL('image/jpeg', 0.85);
		}
	}

	function broadcastPhoto() {
		if (!capturedPhoto || !wsInstance || status !== 'connected') return;
		const photoDataUrl = capturedPhoto;
		wsInstance.sendRaw('photo:share', currentRoom, { dataUrl: photoDataUrl });
		// Also send as a multimedia message in currentRoom so it shows in chat feed
		wsInstance.sendRaw('chat', currentRoom, {
			text: '📸 Shared a photo snapshot',
			files: [
				{
					id: Math.random().toString(36).substring(2, 9),
					name: `Snapshot-${Date.now()}.jpg`,
					size: 120000,
					formattedSize: '120 KB',
					type: 'image/jpeg',
					dataUrl: photoDataUrl,
					isImage: true,
					isAudio: false,
					isVideo: false
				}
			],
			timestamp: Date.now()
		});
		addLog('out', `Photo snapshot shared to #${currentRoom}`);
		showToast(`📸 Photo broadcast to #${currentRoom}!`, 'success');
		capturedPhoto = null;
	}

	async function startCallWith(user: UserProfile) {
		activeCallPeer = user;
		isInCall = true;
		activeTab = 'video';
		await initCamera();

		// If calling a demo peer or bot, simulate interactive peer video answer!
		if (user.id === 'sys-bot' || user.id === 'user-sarah' || user.id === 'user-alex') {
			showToast(`📞 Calling ${user.username}...`, 'info');
			addLog('out', `Calling simulated peer ${user.username}...`);
			setTimeout(() => {
				if (!isInCall || activeCallPeer?.id !== user.id) return;
				const simulatedRemote = createSimulatedMediaStream(user.avatar, user.username);
				if (remoteVideoElem) {
					remoteVideoElem.srcObject = simulatedRemote;
				}
				showToast(`🎉 Call Connected with ${user.username}! (Interactive Stream)`, 'success');
				addLog('sys', `Simulated peer call connected with ${user.username}`);
			}, 700);
			return;
		}

		peerConnection = new RTCPeerConnection({
			iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
		});

		if (localMediaStream) {
			localMediaStream.getTracks().forEach((track) => peerConnection!.addTrack(track, localMediaStream!));
		}

		peerConnection.ontrack = (event) => {
			if (remoteVideoElem && event.streams[0]) {
				remoteVideoElem.srcObject = event.streams[0];
			}
		};

		peerConnection.onicecandidate = (event) => {
			if (event.candidate && wsInstance) {
				wsInstance.sendObj({
					title: 'signal:ice',
					id: user.id,
					payload: { candidate: event.candidate }
				});
			}
		};

		const offer = await peerConnection.createOffer();
		await peerConnection.setLocalDescription(offer);

		wsInstance?.sendObj({
			title: 'signal:call',
			id: user.id,
			payload: { offer }
		});
		addLog('out', `Calling ${user.username}...`);
	}

	async function answerIncomingCall() {
		if (!incomingCall || !wsInstance) return;
		const callerId = incomingCall.fromId;
		const callerName = incomingCall.fromName;
		const offer = incomingCall.offer;
		incomingCall = null;

		isInCall = true;
		activeCallPeer = { id: callerId, username: callerName, avatar: '👤', online: true };
		activeTab = 'video';
		await initCamera();

		peerConnection = new RTCPeerConnection({
			iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
		});

		if (localMediaStream) {
			localMediaStream.getTracks().forEach((track) => peerConnection!.addTrack(track, localMediaStream!));
		}

		peerConnection.ontrack = (event) => {
			if (remoteVideoElem && event.streams[0]) {
				remoteVideoElem.srcObject = event.streams[0];
			}
		};

		peerConnection.onicecandidate = (event) => {
			if (event.candidate && wsInstance) {
				wsInstance.sendObj({
					title: 'signal:ice',
					id: callerId,
					payload: { candidate: event.candidate }
				});
			}
		};

		await peerConnection.setRemoteDescription(new RTCSessionDescription(offer));
		const answer = await peerConnection.createAnswer();
		await peerConnection.setLocalDescription(answer);

		wsInstance.sendObj({
			title: 'signal:answer',
			id: callerId,
			payload: { answer }
		});
		addLog('out', `Answered call from ${callerName}`);
	}

	function rejectIncomingCall() {
		if (incomingCall && wsInstance) {
			wsInstance.sendObj({
				title: 'signal:hangup',
				id: incomingCall.fromId
			});
		}
		incomingCall = null;
	}

	function endCall(notifyPeer: boolean = true) {
		if (notifyPeer && activeCallPeer && wsInstance) {
			wsInstance.sendObj({
				title: 'signal:hangup',
				id: activeCallPeer.id
			});
		}
		if (peerConnection) {
			peerConnection.close();
			peerConnection = null;
		}
		if (remoteVideoElem) {
			remoteVideoElem.srcObject = null;
		}
		isInCall = false;
		activeCallPeer = null;
	}

	// Mini-Game Actions
	function makeGameMove(index: number) {
		if (!wsInstance || status !== 'connected' || game.board[index] !== '' || game.winner !== '') return;
		wsInstance.sendRaw('game:move', null, { index });
	}

	function resetGame() {
		if (!wsInstance || status !== 'connected') return;
		wsInstance.sendRaw('game:reset');
	}

	// Request-Reply Execution
	async function executeRequest() {
		if (!wsInstance || status !== 'connected') return;
		reqLoading = true;
		reqResult = null;

		let payload: any = reqPayload;
		try {
			payload = JSON.parse(reqPayload);
		} catch {
			// keep as text
		}

		const start = performance.now();
		try {
			const res = await wsInstance.request(reqTitle, payload, { timeout: reqTimeout });
			reqResult = {
				status: 'success',
				latencyMs: Math.round(performance.now() - start),
				data: res.payload
			};
		} catch (err: any) {
			reqResult = {
				status: 'error',
				latencyMs: Math.round(performance.now() - start),
				data: err.message || String(err)
			};
		} finally {
			reqLoading = false;
		}
	}

	async function measurePing() {
		if (!wsInstance || status !== 'connected') return;
		const start = performance.now();
		try {
			await wsInstance.request('ping', null, { timeout: 2000 });
			pingMs = Math.round(performance.now() - start);
		} catch {
			pingMs = null;
		}
	}

	onMount(() => {
		loadSavedIdentities();
		loadCachedMessagesBackup();
		connectWebSocket();
		const pingInterval = setInterval(measurePing, 5000);

		return () => {
			clearInterval(pingInterval);
			wsInstance?.closeSafe();
			if (localMediaStream) {
				localMediaStream.getTracks().forEach((t) => t.stop());
			}
		};
	});

	let filteredLogs = $derived(
		logs.filter((l) => (logFilter === 'all' ? true : l.type === logFilter))
	);
</script>

<div class="app-container">
	<!-- Toast Floating Notification -->
	{#if toastNotification}
		<div class="toast-notification {toastNotification.type}">
			{toastNotification.text}
		</div>
	{/if}

	<!-- Top Bar -->
	<header class="header">
		<div class="brand">
			<span class="logo">⚡</span>
			<h1>GinWS Hub</h1>
			<span class="version-tag">Realtime Svelte 5</span>
		</div>

		<div class="header-right">
			<div class="status-indicator {status}">
				<span class="dot"></span>
				<span class="status-text">{status}</span>
				{#if pingMs !== null}
					<span class="ping-badge">{pingMs}ms</span>
				{/if}
			</div>

			{#if isLoggedIn && myProfile}
				<div class="profile-badge">
					<span class="avatar">{myProfile.avatar}</span>
					<span class="username">{myProfile.username}</span>
					{#if myProfile.status}
						<span class="persona-tag">{myProfile.status}</span>
					{/if}
					<span class="client-id">({myClientId})</span>
				</div>
				<button class="btn btn-sm btn-outline-switch" onclick={logoutAndSwitchIdentity} title="Switch or choose another persona">
					🔄 Switch Persona
				</button>
			{:else}
				<button class="btn btn-primary btn-sm" onclick={() => (isLoggedIn = false)}>
					🔑 Select Persona
				</button>
			{/if}
		</div>
	</header>

	<!-- Nav Tabs -->
	<nav class="nav-tabs">
		<button class="tab {activeTab === 'chat' ? 'active' : ''}" onclick={() => (activeTab = 'chat')}>
			💬 Channels & Wall
		</button>
		<button class="tab {activeTab === 'dm' ? 'active' : ''}" onclick={() => (activeTab = 'dm')}>
			📬 Direct Messages
			{#if totalUnreadDms > 0}
				<span class="tab-badge">{totalUnreadDms}</span>
			{/if}
		</button>
		<button class="tab {activeTab === 'wall' ? 'active' : ''}" onclick={() => (activeTab = 'wall')}>
			📢 Public Wall ({wallPosts.length})
		</button>
		<button class="tab {activeTab === 'video' ? 'active' : ''}" onclick={() => { activeTab = 'video'; initCamera(); }}>
			📸 Photo & Video Place
		</button>
		<button class="tab {activeTab === 'game' ? 'active' : ''}" onclick={() => (activeTab = 'game')}>
			🎮 Mini-Game Arena
		</button>
		<button class="tab {activeTab === 'request' ? 'active' : ''}" onclick={() => (activeTab = 'request')}>
			⚡ Request-Reply
		</button>
		<button class="tab {activeTab === 'logs' ? 'active' : ''}" onclick={() => (activeTab = 'logs')}>
			🔍 Logs ({logs.length})
		</button>
	</nav>

	<!-- Incoming Call Alert Modal -->
	{#if incomingCall}
		<div class="modal-overlay">
			<div class="call-modal">
				<div class="calling-avatar">{incomingCall.fromAvatar}</div>
				<h3>{incomingCall.fromName}</h3>
				<p>Incoming real-time video call...</p>
				<div class="call-buttons">
					<button class="btn btn-success btn-large" onclick={answerIncomingCall}>
						📞 Accept
					</button>
					<button class="btn btn-danger btn-large" onclick={rejectIncomingCall}>
						❌ Decline
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Owner Group Permission Requests Modal -->
	{#if showOwnerRequestsModal && pendingPermissionRequests.length > 0}
		<div class="modal-overlay">
			<div class="card request-modal">
				<div class="modal-header">
					<h3>🔔 Pending Join Requests ({pendingPermissionRequests.length})</h3>
					<button class="btn-close" onclick={() => (showOwnerRequestsModal = false)}>✕</button>
				</div>
				<div class="requests-list">
					{#each pendingPermissionRequests as req}
						<div class="request-item">
							<span class="req-avatar">{req.requesterAvatar}</span>
							<div class="req-info">
								<div class="req-name">{req.requesterName}</div>
								<div class="req-sub">Requests to join <strong>#{req.roomName}</strong> • {req.time}</div>
							</div>
							<div class="req-actions">
								<button class="btn btn-success btn-sm" onclick={() => answerPermissionRequest(req, true)}>
									✅ Approve
								</button>
								<button class="btn btn-danger btn-sm" onclick={() => answerPermissionRequest(req, false)}>
									❌ Decline
								</button>
							</div>
						</div>
					{/each}
				</div>
			</div>
		</div>
	{/if}

	<!-- Protected Group Permission Prompt Modal -->
	{#if showPermissionModal}
		<div class="modal-overlay">
			<div class="card permission-modal">
				<div class="perm-icon">{showPermissionModal.icon || '🔒'}</div>
				<h3>Join #{showPermissionModal.name}</h3>
				<span class="badge badge-warning">🔒 Protected Group</span>
				<p class="perm-topic">{showPermissionModal.topic || 'This private group requires permission from the owner to join.'}</p>
				<p class="perm-creator">Group Owner: <strong>{showPermissionModal.creatorName}</strong></p>

				<div class="perm-actions">
					{#if joinRequestStatus[showPermissionModal.id] === 'pending' || showPermissionModal.pending?.includes(myClientId)}
						<div class="pending-notice">
							<span>⏳ Request Sent • Awaiting Approval...</span>
							<p class="sub-notice">The group owner was notified. You will automatically enter once approved.</p>
						</div>
					{:else if joinRequestStatus[showPermissionModal.id] === 'denied'}
						<div class="denied-notice">
							<span>❌ Your join request was declined by the group owner.</span>
						</div>
						<button class="btn btn-primary" onclick={() => requestGroupPermission(showPermissionModal!)}>
							✋ Request Again
						</button>
					{:else}
						<button class="btn btn-primary btn-large btn-block" onclick={() => requestGroupPermission(showPermissionModal!)}>
							✋ Request Permission to Join
						</button>
					{/if}
					<button class="btn btn-secondary btn-block" onclick={() => (showPermissionModal = null)}>
						Cancel
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Create Channel / Protected Group Modal -->
	{#if showCreateRoomModal}
		<div class="modal-overlay">
			<div class="card create-room-modal">
				<div class="modal-header">
					<h3>Create Channel or Group</h3>
					<button class="btn-close" onclick={() => (showCreateRoomModal = false)}>✕</button>
				</div>

				<div class="form-group">
					<label for="room-name-input">Room / Group Name</label>
					<input
						id="room-name-input"
						type="text"
						placeholder="e.g. secret-squad, ai-innovators"
						bind:value={newRoomForm.name}
						class="input-text"
					/>
				</div>

				<div class="form-group">
					<label for="room-topic-input">Topic / Purpose (Optional)</label>
					<input
						id="room-topic-input"
						type="text"
						placeholder="e.g. Confidential discussions, project work"
						bind:value={newRoomForm.topic}
						class="input-text"
					/>
				</div>

				<div class="form-group">
					<label for="room-icon-picker">Choose Icon</label>
					<div class="icon-picker" id="room-icon-picker">
						{#each ['💬', '🔒', '🚀', '💡', '🛡️', '🍸', '🎨', '🎵', '⚡', '👾'] as ic}
							<button
								type="button"
								class="icon-picker-btn {newRoomForm.icon === ic ? 'selected' : ''}"
								onclick={() => (newRoomForm.icon = ic)}
							>
								{ic}
							</button>
						{/each}
					</div>
				</div>

				<div class="form-group">
					<label for="privacy-mode-group">Access Control & Permissions</label>
					<div class="privacy-selector" id="privacy-mode-group">
						<label class="privacy-option {!newRoomForm.isProtected ? 'selected' : ''}">
							<input
								type="radio"
								name="privacy"
								checked={!newRoomForm.isProtected}
								onchange={() => (newRoomForm.isProtected = false)}
							/>
							<div class="privacy-details">
								<div class="privacy-title">🟢 Public Channel</div>
								<div class="privacy-desc">Open to everyone. Anyone can join and chat immediately.</div>
							</div>
						</label>

						<label class="privacy-option {newRoomForm.isProtected ? 'selected' : ''}">
							<input
								type="radio"
								name="privacy"
								checked={newRoomForm.isProtected}
								onchange={() => (newRoomForm.isProtected = true)}
							/>
							<div class="privacy-details">
								<div class="privacy-title">🔒 Protected Group (Ask Permission)</div>
								<div class="privacy-desc">Users must request and receive your permission to join.</div>
							</div>
						</label>
					</div>
				</div>

				<div class="modal-footer">
					<button
						class="btn btn-primary btn-large btn-block"
						onclick={submitCreateRoom}
						disabled={!newRoomForm.name.trim()}
					>
						{newRoomForm.isProtected ? 'Create Protected Group 🔒' : 'Create Public Channel 🚀'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- PERSONA SELECTION & LOGIN SCREEN (If not logged in) -->
	{#if !isLoggedIn}
		<div class="login-overlay">
			<div class="card login-card persona-modal">
				<div class="login-header">
					<span class="login-logo">⚡</span>
					<h2>Who Do You Want to Be Today?</h2>
					<p>Select an existing persona or create who you are to enter GinWS.</p>
				</div>

				{#if savedIdentities.length > 0}
					<div class="saved-identities-section">
						<div class="saved-identities-title">
							<span>Choose Existing Persona</span>
							<span class="count-badge">{savedIdentities.length}</span>
						</div>
						<div class="saved-identities-grid">
							{#each savedIdentities as identity (identity.username)}
								<div class="saved-identity-card {usernameInput === identity.username ? 'active-card' : ''}">
									<button
										type="button"
										class="saved-identity-action"
										onclick={() => selectSavedIdentity(identity)}
										title="Enter as {identity.username}"
									>
										<span class="saved-identity-avatar">{identity.avatar}</span>
										<div class="saved-identity-meta">
											<span class="saved-identity-name">{identity.username}</span>
											{#if identity.status}
												<span class="saved-identity-status">{identity.status}</span>
											{/if}
											<span class="saved-identity-time">{formatLastUsed(identity.lastUsed)}</span>
										</div>
									</button>
									<button
										type="button"
										class="saved-identity-del"
										onclick={() => removeSavedIdentity(identity.username)}
										title="Forget {identity.username}"
									>
										✕
									</button>
								</div>
							{/each}
						</div>
					</div>

					<div class="login-divider">
						<span>or create who you are</span>
					</div>
				{/if}

				<div class="form-group">
					<label for="login-username">Display Name</label>
					<input
						id="login-username"
						type="text"
						placeholder="e.g. Alice, Bob, Neo"
						bind:value={usernameInput}
						onkeydown={(e) => e.key === 'Enter' && handleLoginSubmit()}
						class="input-text"
					/>
				</div>

				<div class="form-group">
					<label for="login-title">Title / Role / Status (Optional)</label>
					<input
						id="login-title"
						type="text"
						placeholder="e.g. ⚡ Fullstack Dev, 🎮 Pro Gamer, ☕ Coffee Addict"
						bind:value={userTitleInput}
						onkeydown={(e) => e.key === 'Enter' && handleLoginSubmit()}
						class="input-text"
					/>
				</div>

				<div class="form-group">
					<label for="avatar-picker">Choose Your Avatar</label>
					<div class="avatar-picker" id="avatar-picker">
						{#each AVATARS as av}
							<button
								type="button"
								class="avatar-btn {selectedAvatar === av ? 'selected' : ''}"
								onclick={() => (selectedAvatar = av)}
							>
								{av}
							</button>
						{/each}
					</div>
				</div>

				<div class="remember-option">
					<label class="checkbox-label">
						<input type="checkbox" bind:checked={rememberIdentity} />
						<span>Remember this persona in this browser</span>
					</label>
					<label class="checkbox-label" style="margin-top: 6px;">
						<input type="checkbox" bind:checked={autoLoginOnStartup} />
						<span>Auto-login with active persona on startup</span>
					</label>
				</div>

				<button
					class="btn btn-primary btn-large btn-block"
					onclick={handleLoginSubmit}
					disabled={status !== 'connected' || !usernameInput.trim()}
				>
					{status === 'connected' ? `Enter as ${usernameInput.trim() || 'New Persona'} 🚀` : 'Connecting to GinWS...'}
				</button>
			</div>
		</div>
	{/if}

	<!-- Main Workspace Area -->
	<main class="main-body">
		<!-- TAB 1: Real-time Multi-Room Chat -->
		{#if activeTab === 'chat'}
			<div class="chat-workspace">
				<!-- Sidebar: Rooms & Online Users -->
				<aside class="sidebar">
					<!-- Channels & Groups Section -->
					<div class="sidebar-section">
						<div class="sidebar-section-header room-section-header">
							<h3>Channels & Groups</h3>
							<button class="btn btn-sm btn-create-room" onclick={() => (showCreateRoomModal = true)} title="Create Channel or Protected Group">
								+ Create
							</button>
						</div>

						{#if pendingPermissionRequests.length > 0}
							<button class="pending-requests-banner" onclick={() => (showOwnerRequestsModal = true)}>
								🔔 {pendingPermissionRequests.length} Join Request(s)
							</button>
						{/if}

						<div class="list-container">
							{#each (roomList.length > 0 ? roomList : [{ id: 'general', name: 'general', topic: 'General room', icon: '💬', creatorId: 'system', creatorName: 'System', isProtected: false, members: [], pending: [], createdAt: '' }, { id: 'engineering', name: 'engineering', topic: 'Tech discussions', icon: '⚙️', creatorId: 'system', creatorName: 'System', isProtected: false, members: [], pending: [], createdAt: '' }, { id: 'gaming', name: 'gaming', topic: 'Mini-games', icon: '🎮', creatorId: 'system', creatorName: 'System', isProtected: false, members: [], pending: [], createdAt: '' }]) as room (room.id || room.name)}
								{@const isOwner = room.creatorId === myClientId}
								{@const isMember = room.members && room.members.includes(myClientId)}
								{@const isPending = room.pending && room.pending.includes(myClientId)}
								<button
									class="list-item {currentRoom === room.name ? 'active' : ''}"
									onclick={() => selectRoom(room.name)}
								>
									<span class="room-icon">{room.icon || (room.isProtected ? '🔒' : '#')}</span>
									<div class="room-name-meta">
										<span class="item-label">{room.name}</span>
										{#if room.isProtected}
											<span class="room-badge-protected">
												{isOwner ? '👑 Owner' : isMember ? '✓ Member' : isPending ? '⏳ Pending' : '🔒 Group'}
											</span>
										{/if}
									</div>
									{#if roomUnread[room.name] > 0 && currentRoom !== room.name}
										<span class="badge-unread-room" title="{roomUnread[room.name]} unread messages">{roomUnread[room.name]}</span>
									{/if}
									{#if currentRoom === room.name}
										<span class="active-dot">●</span>
									{/if}
								</button>
							{/each}
						</div>
					</div>

					<!-- Online Users Section -->
					<div class="sidebar-section users-section">
						<div class="sidebar-section-header">
							<h3>Online Peers ({onlineUsers.length})</h3>
						</div>
						<div class="list-container">
							{#if onlineUsers.length === 0}
								<p class="empty-hint">Waiting for peers...</p>
							{:else}
								{#each onlineUsers as user}
									<div class="user-row {user.id === myClientId ? 'self-user' : ''}">
										<span class="user-avatar">{user.avatar}</span>
										<div class="user-info">
											<span class="user-name">{user.username}</span>
											{#if user.id === myClientId}
												<span class="you-tag">You</span>
											{/if}
										</div>
										{#if user.id !== myClientId}
											<div class="user-actions">
												{#if unreadDms[user.id] > 0}
													<span class="badge-unread-user" title="{unreadDms[user.id]} unread messages">{unreadDms[user.id]}</span>
												{/if}
												<button class="icon-btn" title="Direct Message {user.username}" onclick={() => openDmWith(user)}>💬</button>
												<button class="icon-btn" title="Video Call {user.username}" onclick={() => startCallWith(user)}>📞</button>
											</div>
										{/if}
									</div>
								{/each}
							{/if}
						</div>
					</div>
				</aside>

				<!-- Chat Message Feed -->
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<section
					class="chat-main {isDraggingChat ? 'drag-over' : ''}"
					ondragover={handleChatDragOver}
					ondragleave={handleChatDragLeave}
					ondrop={handleChatDrop}
				>
					{#if isDraggingChat}
						<div class="drag-drop-overlay">
							<div class="drag-drop-card">
								<span class="drag-drop-icon">📥</span>
								<h3>Drop files to attach</h3>
								<p>Images, audio, video, documents</p>
							</div>
						</div>
					{/if}

					<div class="chat-header">
						<div class="current-room-info">
							<span class="room-icon-title">{currentRoomData?.icon || '#'}</span>
							<span class="room-title">{currentRoom}</span>
							{#if currentRoomData?.isProtected}
								<span class="badge badge-warning">🔒 Protected Group</span>
							{/if}
							<span class="room-sub">{currentRoomData?.topic || 'Real-time room broadcasting'}</span>
						</div>
						<div class="chat-header-actions">
							<button
								type="button"
								class="btn btn-sm btn-outline-recovery"
								onclick={simulateOfflineRecovery}
								disabled={isSimulatingOffline}
								title="Simulate dropping offline for 2.5s and automatically catching up on all messages"
							>
								{isSimulatingOffline ? '⏳ Reconnecting...' : '⚡ Test Offline Sync'}
							</button>
							{#if currentRoomData?.creatorId === myClientId && pendingPermissionRequests.some((r) => r.roomId === currentRoomData?.id)}
								<button class="btn btn-warning btn-sm" onclick={() => (showOwnerRequestsModal = true)}>
									🔔 {pendingPermissionRequests.filter((r) => r.roomId === currentRoomData?.id).length} Join Request(s)
								</button>
							{/if}
						</div>
					</div>

					<!-- Live Public Wall & Broadcast Banner (Unified Architecture) -->
					<div class="public-wall-banner">
						<div class="wall-banner-header">
							<div class="wall-banner-title">
								<span class="wall-pulse">●</span>
								<span class="wall-title-text">📢 Public Wall & Live Broadcast</span>
								{#if wallPosts.length > 0}
									<span class="wall-latest-preview">
										<strong>{wallPosts[0].authorAvatar} {wallPosts[0].authorName}:</strong> "{wallPosts[0].content}"
									</span>
								{/if}
							</div>
							<div class="wall-banner-controls">
								<button class="btn btn-xs btn-outline-banner" onclick={() => (showWallBanner = !showWallBanner)}>
									{showWallBanner ? '▲ Hide' : '📢 Shout ▼'}
								</button>
								<button class="btn btn-xs btn-wall-link" onclick={() => selectRoom('public-wall')}>
									View All Shouts ({wallPosts.length}) ➔
								</button>
							</div>
						</div>
						{#if showWallBanner}
							<div class="wall-quick-composer">
								<input
									type="text"
									placeholder="Broadcast a shout or announcement live to all peers..."
									bind:value={quickWallInput}
									onkeydown={(e) => {
										if (e.key === 'Enter' && quickWallInput.trim()) {
											submitQuickWallPost();
										}
									}}
								/>
								<button
									class="btn btn-primary btn-sm"
									onclick={submitQuickWallPost}
									disabled={!quickWallInput.trim() || status !== 'connected'}
								>
									Broadcast 🚀
								</button>
							</div>
						{/if}
					</div>

					<div class="messages-container" bind:this={chatBoxElement}>
						{#if currentRoomMessages.length === 0}
							<div class="empty-state">
								<p>No messages in #{currentRoom} yet. Say hello!</p>
							</div>
						{:else}
							{#each currentRoomMessages as msg (msg.id)}
								{#if msg.recalled}
									<div class="message-bubble recalled {msg.isSelf ? 'self' : 'other'}">
										<div class="bubble-meta">
											<span class="bubble-avatar">{msg.senderAvatar || '👤'}</span>
											<span class="bubble-sender">{msg.isSelf ? 'You' : msg.senderName || msg.sender}</span>
											<span class="bubble-time">{msg.time}</span>
										</div>
										<div class="bubble-content recalled-content">
											<span class="recalled-icon">↩️</span>
											<span class="recalled-text">This message was recalled</span>
										</div>
									</div>
								{:else}
									<div class="message-bubble {msg.isSelf ? 'self' : 'other'}">
										<div class="bubble-meta">
											<span class="bubble-avatar">{msg.senderAvatar || '👤'}</span>
											<span class="bubble-sender">{msg.isSelf ? 'You' : msg.senderName || msg.sender}</span>
											<span class="bubble-time">{msg.time}</span>
											{#if msg.isSelf}
												<button
													type="button"
													class="btn-recall-msg"
													onclick={() => recallMessage(msg.id)}
													title="Recall (unsend) this message for everyone"
												>
													↩️ Recall
												</button>
											{/if}
										</div>
										{#if msg.text}
											<div class="bubble-content">{msg.text}</div>
										{/if}
										{#if msg.files && msg.files.length > 0}
											<div class="bubble-attachments">
												{#each msg.files as file (file.id)}
													{#if file.isImage}
														<div class="attachment-media image-attachment">
															<a href={file.dataUrl} target="_blank" rel="noreferrer" title="Click to view full image">
																<img src={file.dataUrl} alt={file.name} class="attachment-img" />
															</a>
															<div class="attachment-meta">
																<span class="attachment-name">{file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else if file.isAudio}
														<div class="attachment-media audio-attachment">
															<!-- svelte-ignore a11y_media_has_caption -->
															<audio controls src={file.dataUrl} class="attachment-audio"></audio>
															<div class="attachment-meta">
																<span class="attachment-name">🎵 {file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else if file.isVideo}
														<div class="attachment-media video-attachment">
															<!-- svelte-ignore a11y_media_has_caption -->
															<video controls src={file.dataUrl} class="attachment-video"></video>
															<div class="attachment-meta">
																<span class="attachment-name">🎥 {file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else}
														<div class="attachment-card">
															<span class="attachment-file-icon">📄</span>
															<div class="attachment-card-info">
																<span class="attachment-name">{file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
															<a href={file.dataUrl} download={file.name} class="btn btn-sm btn-download">
																⬇ Download
															</a>
														</div>
													{/if}
												{/each}
											</div>
										{/if}

										<!-- Facebook-Style Read Receipts -->
										{#if msg.readBy && msg.readBy.filter((r) => r.userId !== msg.sender).length > 0}
											<div class="read-receipts-row" title="Seen by {msg.readBy.filter((r) => r.userId !== msg.sender).map((r) => r.username).join(', ')}">
												<span class="seen-label">Seen</span>
												<div class="seen-avatars">
													{#each msg.readBy.filter((r) => r.userId !== msg.sender) as reader (reader.userId)}
														<span class="seen-avatar-badge" title="Seen by {reader.username} at {reader.readAt}">
															{reader.avatar}
														</span>
													{/each}
												</div>
											</div>
										{/if}
									</div>
								{/if}
							{/each}
						{/if}
					</div>

					<div class="chat-input-wrapper">
						{#if chatAttachedFiles.length > 0}
							<div class="attachment-preview-shelf">
								{#each chatAttachedFiles as file (file.id)}
									<div class="attachment-chip">
										{#if file.isImage}
											<img src={file.dataUrl} alt={file.name} class="chip-thumb" />
										{:else if file.isAudio}
											<span class="chip-icon">🎵</span>
										{:else if file.isVideo}
											<span class="chip-icon">🎥</span>
										{:else}
											<span class="chip-icon">📄</span>
										{/if}
										<div class="chip-meta">
											<span class="chip-name">{file.name}</span>
											<span class="chip-size">{file.formattedSize}</span>
										</div>
										<button
											type="button"
											class="chip-remove"
											onclick={() => removeChatAttachment(file.id)}
											title="Remove attachment"
										>
											✕
										</button>
									</div>
								{/each}
							</div>
						{/if}

						<div class="chat-input-bar">
							<input
								type="file"
								multiple
								bind:this={chatFileInputElem}
								onchange={handleChatFileInput}
								style="display: none;"
							/>
							<button
								type="button"
								class="btn-icon-attach"
								onclick={() => chatFileInputElem?.click()}
								title="Attach multimedia files (or drag and drop)"
							>
								📎
							</button>
							<input
								type="text"
								placeholder={chatAttachedFiles.length > 0 ? "Add a message (optional)..." : `Message #${currentRoom}... (or drag files)`}
								bind:value={chatInput}
								onkeydown={(e) => e.key === 'Enter' && sendChatMessage()}
							/>
							<button
								class="btn btn-primary"
								onclick={sendChatMessage}
								disabled={status !== 'connected' || (!chatInput.trim() && chatAttachedFiles.length === 0)}
							>
								Send
							</button>
						</div>
					</div>
				</section>
			</div>

		<!-- TAB 2: Direct Messages (Knowing Who To Send To) -->
		{:else if activeTab === 'dm'}
			<div class="dm-workspace">
				<!-- Online Users to DM -->
				<aside class="sidebar">
					<div class="sidebar-section-header">
						<h3>Direct Messages</h3>
					</div>
					<div class="list-container">
						{#if onlineUsers.filter((u) => u.id !== myClientId).length === 0}
							<p class="empty-hint">Waiting for peers to connect...</p>
						{:else}
							{#each onlineUsers.filter((u) => u.id !== myClientId) as user}
								<button
									class="dm-user-item {selectedDmUser?.id === user.id ? 'active' : ''}"
									onclick={() => openDmWith(user)}
								>
									<span class="avatar">{user.avatar}</span>
									<div class="dm-user-meta">
										<span class="name">{user.username}</span>
										<span class="id">{user.status || user.id}</span>
									</div>
									{#if unreadDms[user.id] > 0}
										<span class="dm-unread-badge">{unreadDms[user.id]}</span>
									{/if}
								</button>
							{/each}
						{/if}
					</div>
				</aside>

				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<section
					class="chat-main {isDraggingDm ? 'drag-over' : ''}"
					ondragover={handleDmDragOver}
					ondragleave={handleDmDragLeave}
					ondrop={handleDmDrop}
				>
					{#if isDraggingDm}
						<div class="drag-drop-overlay">
							<div class="drag-drop-card">
								<span class="drag-drop-icon">📥</span>
								<h3>Drop files to attach</h3>
								<p>Images, audio, video, documents</p>
							</div>
						</div>
					{/if}

					{#if selectedDmUser}
						<div class="chat-header">
							<div class="current-room-info">
								<span class="avatar">{selectedDmUser.avatar}</span>
								<span class="room-title">{selectedDmUser.username}</span>
								<span class="room-sub">Private 1-on-1 direct message</span>
							</div>
							<button class="btn btn-success btn-sm" onclick={() => startCallWith(selectedDmUser!)}>
								📞 Video Call
							</button>
						</div>

						<div class="messages-container" bind:this={dmBoxElement}>
							{#if !dmConversations[selectedDmUser.id] || dmConversations[selectedDmUser.id]!.length === 0}
								<div class="empty-state">
									<p>No messages with {selectedDmUser.username} yet. Send a direct message!</p>
								</div>
							{:else}
								{#each dmConversations[selectedDmUser.id]! as dm (dm.id)}
									<div class="message-bubble {dm.isSelf ? 'self' : 'other'}">
										<div class="bubble-meta">
											<span class="bubble-avatar">{dm.fromAvatar}</span>
											<span class="bubble-sender">{dm.isSelf ? 'You' : dm.fromName}</span>
											<span class="bubble-time">{dm.time}</span>
										</div>
										{#if dm.text}
											<div class="bubble-content">{dm.text}</div>
										{/if}
										{#if dm.files && dm.files.length > 0}
											<div class="bubble-attachments">
												{#each dm.files as file (file.id)}
													{#if file.isImage}
														<div class="attachment-media image-attachment">
															<a href={file.dataUrl} target="_blank" rel="noreferrer" title="Click to view full image">
																<img src={file.dataUrl} alt={file.name} class="attachment-img" />
															</a>
															<div class="attachment-meta">
																<span class="attachment-name">{file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else if file.isAudio}
														<div class="attachment-media audio-attachment">
															<!-- svelte-ignore a11y_media_has_caption -->
															<audio controls src={file.dataUrl} class="attachment-audio"></audio>
															<div class="attachment-meta">
																<span class="attachment-name">🎵 {file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else if file.isVideo}
														<div class="attachment-media video-attachment">
															<!-- svelte-ignore a11y_media_has_caption -->
															<video controls src={file.dataUrl} class="attachment-video"></video>
															<div class="attachment-meta">
																<span class="attachment-name">🎥 {file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
														</div>
													{:else}
														<div class="attachment-card">
															<span class="attachment-file-icon">📄</span>
															<div class="attachment-card-info">
																<span class="attachment-name">{file.name}</span>
																<span class="attachment-size">{file.formattedSize}</span>
															</div>
															<a href={file.dataUrl} download={file.name} class="btn btn-sm btn-download">
																⬇ Download
															</a>
														</div>
													{/if}
												{/each}
											</div>
										{/if}

										{#if dm.isSelf}
											<div class="dm-status-row">
												{#if dm.read}
													<span class="dm-read-status" title="Seen at {dm.readAt || ''}">
														✓ Seen {selectedDmUser.avatar}
													</span>
												{:else}
													<span class="dm-sent-status">✓ Sent</span>
												{/if}
											</div>
										{/if}
									</div>
								{/each}
							{/if}
						</div>

						<div class="chat-input-wrapper">
							{#if dmAttachedFiles.length > 0}
								<div class="attachment-preview-shelf">
									{#each dmAttachedFiles as file (file.id)}
										<div class="attachment-chip">
											{#if file.isImage}
												<img src={file.dataUrl} alt={file.name} class="chip-thumb" />
											{:else if file.isAudio}
												<span class="chip-icon">🎵</span>
											{:else if file.isVideo}
												<span class="chip-icon">🎥</span>
											{:else}
												<span class="chip-icon">📄</span>
											{/if}
											<div class="chip-meta">
												<span class="chip-name">{file.name}</span>
												<span class="chip-size">{file.formattedSize}</span>
											</div>
											<button
												type="button"
												class="chip-remove"
												onclick={() => removeDmAttachment(file.id)}
												title="Remove attachment"
											>
												✕
											</button>
										</div>
									{/each}
								</div>
							{/if}

							<div class="chat-input-bar">
								<input
									type="file"
									multiple
									bind:this={dmFileInputElem}
									onchange={handleDmFileInput}
									style="display: none;"
								/>
								<button
									type="button"
									class="btn-icon-attach"
									onclick={() => dmFileInputElem?.click()}
									title="Attach multimedia files (or drag and drop)"
								>
									📎
								</button>
								<input
									type="text"
									placeholder={dmAttachedFiles.length > 0 ? "Add a message (optional)..." : `Send message to ${selectedDmUser.username}... (or drag files)`}
									bind:value={dmInput}
									onkeydown={(e) => e.key === 'Enter' && sendDm()}
								/>
								<button
									class="btn btn-primary"
									onclick={sendDm}
									disabled={status !== 'connected' || (!dmInput.trim() && dmAttachedFiles.length === 0)}
								>
									Send
								</button>
							</div>
						</div>
					{:else}
						<div class="empty-state dm-empty-picker">
							<span class="empty-icon" style="font-size: 2.5rem; margin-bottom: 8px;">📬</span>
							<h3>Start a Direct Message or Video Call</h3>
							<p style="color: #8b949e; margin-bottom: 16px;">Choose an online peer or bot below to start a private conversation or 1-on-1 call:</p>
							<div class="dm-quick-peers-grid">
								{#each onlineUsers.filter((u) => u.id !== myClientId) as peer}
									<div class="dm-quick-peer-card card">
										<span class="peer-avatar" style="font-size: 2rem;">{peer.avatar}</span>
										<div class="peer-info">
											<span class="peer-name" style="font-weight: 600; color: #f0f6fc;">{peer.username}</span>
											<span class="peer-status" style="font-size: 0.75rem; color: #8b949e;">{peer.status || peer.id}</span>
										</div>
										<div class="peer-btns" style="display: flex; gap: 6px; margin-top: 10px;">
											<button class="btn btn-sm btn-primary" onclick={() => openDmWith(peer)}>💬 Chat</button>
											<button class="btn btn-sm btn-success" onclick={() => startCallWith(peer)}>📞 Call</button>
										</div>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</section>
			</div>

		<!-- TAB 3: The Public Broadcast Wall -->
		{:else if activeTab === 'wall'}
			<div class="wall-workspace">
				<div class="wall-container">
					<div class="wall-composer card">
						<h2>📢 The Public Wall</h2>
						<p class="section-desc">Post announcements, thoughts, or shouts that broadcast live to everyone!</p>
						<textarea
							rows="3"
							placeholder="What's happening? Post publicly to everyone..."
							bind:value={wallInput}
							class="input-textarea"
						></textarea>
						<div class="composer-footer">
							<button class="btn btn-primary" onclick={submitWallPost} disabled={!wallInput.trim()}>
								Broadcast to Wall 🚀
							</button>
						</div>
					</div>

					<div class="wall-feed">
						{#if wallPosts.length === 0}
							<div class="empty-state"><p>No posts on the wall yet. Be the first to shout!</p></div>
						{:else}
							{#each wallPosts as post (post.id)}
								<div class="wall-post-card card">
									<div class="post-header">
										<span class="post-avatar">{post.authorAvatar}</span>
										<div class="post-author-info">
											<span class="post-author-name">{post.authorName}</span>
											<span class="post-time">{post.time}</span>
										</div>
									</div>
									<div class="post-content">{post.content}</div>
								</div>
							{/each}
						{/if}
					</div>
				</div>
			</div>

		<!-- TAB 4: Video Call & Photo-to-Photo ("phot to phot") -->
		{:else if activeTab === 'video'}
			<div class="video-workspace">
				<!-- Direct Peer Calling Bar -->
				<div class="video-peer-launcher card">
					<div class="launcher-header">
						<span class="launcher-title">⚡ Instant 1-on-1 WebRTC Video Calling</span>
						<span class="sub-hint">Click any online peer below to connect immediately:</span>
					</div>
					<div class="launcher-chips">
						{#each onlineUsers.filter((u) => u.id !== myClientId) as peer}
							<button class="peer-call-chip {activeCallPeer?.id === peer.id ? 'calling-chip' : ''}" onclick={() => startCallWith(peer)}>
								<span class="chip-av">{peer.avatar}</span>
								<span class="chip-name">{peer.username}</span>
								<span class="chip-call-icon">📞 Call</span>
							</button>
						{/each}
					</div>
				</div>

				<div class="video-grid">
					<!-- Local Stream / Camera Preview -->
					<div class="video-box card">
						<div class="video-box-header">
							<h3>📹 Your Camera Preview</h3>
							{#if !localMediaStream}
								<button class="btn btn-sm btn-primary" onclick={initCamera}>Enable Camera</button>
							{:else}
								<button class="btn btn-xs btn-outline" onclick={() => { localMediaStream = null; initCamera(); }}>
									🔄 Switch Feed
								</button>
							{/if}
						</div>
						<div class="video-viewport">
							<video bind:this={localVideoElem} autoplay playsinline muted class="video-feed"></video>
						</div>
						<div class="video-controls">
							<button class="btn btn-sm" onclick={snapPhoto} disabled={!localMediaStream}>
								📸 Snap Photo
							</button>
							{#if capturedPhoto}
								<button class="btn btn-sm btn-success" onclick={broadcastPhoto}>
									🚀 Share Photo to Room
								</button>
							{/if}
						</div>
						{#if capturedPhoto}
							<div class="snapshot-preview">
								<p>Snapshot ready to send:</p>
								<img src={capturedPhoto} alt="Snapshot preview" class="preview-img" />
							</div>
						{/if}
					</div>

					<!-- Remote Peer Video -->
					<div class="video-box card">
						<div class="video-box-header">
							<h3>
								👥 {isInCall && activeCallPeer ? `${activeCallPeer.username}'s Stream` : 'Peer Video Call'}
							</h3>
							{#if isInCall}
								<button class="btn btn-sm btn-danger" onclick={() => endCall(true)}>End Call</button>
							{/if}
						</div>
						<div class="video-viewport remote">
							<video bind:this={remoteVideoElem} autoplay playsinline class="video-feed"></video>
							{#if !isInCall}
								<div class="remote-placeholder">
									<p>Not in a call currently.</p>
									<p class="sub">Click 📞 next to any online user in the sidebar to start a call!</p>
								</div>
							{/if}
						</div>
					</div>
				</div>

				<!-- Received Photos Feed ("phot to phot") -->
				{#if receivedPhotos.length > 0}
					<div class="photos-feed card">
						<h3>📸 Realtime Photo Stream</h3>
						<div class="photo-gallery">
							{#each receivedPhotos as p}
								<div class="photo-card">
									<img src={p.dataUrl} alt="Received from {p.sender}" />
									<div class="photo-meta">
										<span>{p.avatar} {p.sender}</span>
										<span class="photo-time">{p.time}</span>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</div>

		<!-- TAB 5: Multiplayer Mini-Game (Tic-Tac-Toe via Chaining) -->
		{:else if activeTab === 'game'}
			<div class="game-workspace">
				<div class="game-card card">
					<div class="game-header">
						<h2>🎮 Real-Time Tic-Tac-Toe Arena</h2>
						<p class="section-desc">
							Multiplayer synchronized game powered by the 3-step action chain:
							<code>game:move</code> ➔ <code>game:validate</code> ➔ <code>game:evaluate</code>
						</p>
					</div>

					<div class="game-scoreboard">
						<div class="score-pill {game.turn === 'X' ? 'active' : ''}">
							Player X (You): <span class="score-val">{game.scoreX}</span>
						</div>
						<div class="turn-indicator">
							{#if game.winner}
								<span class="winner-announcement">
									{game.winner === 'Draw' ? "It's a Draw! 🤝" : `Winner: ${game.winner}! 🎉`}
								</span>
							{:else}
								<span>Turn: <strong>{game.turn}</strong></span>
								{#if isAiThinking}
									<span class="ai-thinking-pill">🤖 AI Opponent is replying...</span>
								{/if}
							{/if}
						</div>
						<div class="score-pill {game.turn === 'O' ? 'active' : ''}">
							Player O (Bot): <span class="score-val">{game.scoreO}</span>
						</div>
					</div>

					<div class="game-board">
						{#each game.board as cell, i}
							<button
								class="game-cell {cell} {game.winner || game.turn !== 'X' ? 'disabled' : ''}"
								onclick={() => makeGameMove(i)}
								disabled={cell !== '' || game.winner !== '' || game.turn !== 'X'}
							>
								{cell}
							</button>
						{/each}
					</div>

					<div class="game-footer">
						<button class="btn btn-primary" onclick={resetGame}>Reset Board 🔄</button>
						<span class="game-auto-reply-note">⚡ Automated AI opponent response active on turn O via action chaining</span>
					</div>
				</div>
			</div>

		<!-- TAB 6: Request-Reply Console -->
		{:else if activeTab === 'request'}
			<div class="request-workspace">
				<div class="card request-form-card">
					<h2>Request-Reply Console</h2>
					<p class="section-desc">Directly test <code>await ws.request()</code> promises.</p>

					<div class="form-group">
						<label for="req-action">Action Title</label>
						<div class="preset-buttons">
							<button class="chip" onclick={() => (reqTitle = 'ping')}>ping</button>
							<button class="chip" onclick={() => (reqTitle = 'echo')}>echo</button>
							<button class="chip" onclick={() => (reqTitle = 'hello')}>hello</button>
							<button class="chip chip-warn" onclick={() => { reqTitle = 'slow_echo'; reqTimeout = 1000; }}>
								slow_echo (timeout test)
							</button>
						</div>
						<input id="req-action" type="text" bind:value={reqTitle} class="input-text" />
					</div>

					<div class="form-group">
						<label for="req-payload">JSON / String Payload</label>
						<textarea id="req-payload" rows="4" bind:value={reqPayload} class="input-textarea"></textarea>
					</div>

					<div class="form-group">
						<label for="req-timeout">Timeout (ms)</label>
						<input id="req-timeout" type="number" bind:value={reqTimeout} class="input-number" />
					</div>

					<button class="btn btn-primary btn-large" onclick={executeRequest} disabled={reqLoading}>
						{reqLoading ? 'Awaiting Reply...' : 'Execute ws.request()'}
					</button>
				</div>

				<div class="card response-card">
					<h2>Server Reply Inspector</h2>
					{#if reqResult}
						<div class="response-result {reqResult.status}">
							<div class="result-header">
								<span class="badge {reqResult.status}">{reqResult.status.toUpperCase()}</span>
								<span class="latency-stat">⏱ {reqResult.latencyMs}ms roundtrip</span>
							</div>
							<pre class="code-preview">{JSON.stringify(reqResult.data, null, 2)}</pre>
						</div>
					{:else}
						<div class="empty-state"><p>Send a request to inspect response.</p></div>
					{/if}
				</div>
			</div>

		<!-- TAB 7: Diagnostic Logs -->
		{:else if activeTab === 'logs'}
			<div class="logs-workspace">
				<div class="logs-toolbar">
					<div class="filter-group">
						<button class="filter-chip {logFilter === 'all' ? 'active' : ''}" onclick={() => (logFilter = 'all')}>All</button>
						<button class="filter-chip {logFilter === 'in' ? 'active' : ''}" onclick={() => (logFilter = 'in')}>Inbound</button>
						<button class="filter-chip {logFilter === 'out' ? 'active' : ''}" onclick={() => (logFilter = 'out')}>Outbound</button>
						<button class="filter-chip {logFilter === 'err' ? 'active' : ''}" onclick={() => (logFilter = 'err')}>Errors</button>
					</div>
					<button class="btn btn-sm" onclick={() => (logs = [])}>Clear Logs</button>
				</div>

				<div class="logs-list">
					{#each filteredLogs as log (log.id)}
						<div class="log-row {log.type}">
							<span class="log-time">{log.time}</span>
							<span class="log-type-tag">{log.type.toUpperCase()}</span>
							<span class="log-summary">{log.summary}</span>
							{#if log.details}
								<details class="log-details">
									<summary>Payload</summary>
									<pre>{JSON.stringify(log.details, null, 2)}</pre>
								</details>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</main>
</div>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
		background: #0d1117;
		color: #c9d1d9;
		height: 100vh;
		overflow: hidden;
	}

	.app-container {
		display: flex;
		flex-direction: column;
		height: 100vh;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 10px 20px;
		background: #161b22;
		border-bottom: 1px solid #30363d;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.brand h1 {
		font-size: 1.15rem;
		margin: 0;
		color: #f0f6fc;
	}

	.version-tag {
		font-size: 0.7rem;
		background: #238636;
		color: white;
		padding: 2px 6px;
		border-radius: 10px;
	}

	.header-right {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.profile-badge {
		display: flex;
		align-items: center;
		gap: 6px;
		background: #21262d;
		border: 1px solid #30363d;
		padding: 4px 10px;
		border-radius: 14px;
		font-size: 0.85rem;
	}

	.client-id {
		color: #8b949e;
		font-size: 0.75rem;
		font-family: monospace;
	}

	.btn-outline-switch {
		background: #21262d;
		border: 1px solid #30363d;
		color: #c9d1d9;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		font-size: 0.8rem;
		border-radius: 6px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.btn-outline-switch:hover {
		background: #30363d;
		border-color: #58a6ff;
		color: #58a6ff;
	}

	.status-indicator {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		border-radius: 12px;
		font-size: 0.8rem;
		background: #21262d;
	}

	.status-indicator .dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.status-indicator.connected .dot { background: #3fb950; box-shadow: 0 0 6px #3fb950; }
	.status-indicator.connecting .dot { background: #d29922; }
	.status-indicator.disconnected .dot { background: #f85149; }

	.ping-badge {
		font-size: 0.75rem;
		color: #58a6ff;
	}

	.nav-tabs {
		display: flex;
		background: #161b22;
		border-bottom: 1px solid #30363d;
		padding: 0 16px;
		overflow-x: auto;
	}

	.tab {
		background: transparent;
		border: none;
		border-bottom: 2px solid transparent;
		color: #8b949e;
		padding: 10px 16px;
		font-size: 0.85rem;
		font-weight: 500;
		cursor: pointer;
		white-space: nowrap;
	}

	.tab:hover { color: #f0f6fc; }
	.tab.active { color: #58a6ff; border-bottom-color: #58a6ff; }

	.main-body {
		flex: 1;
		overflow: hidden;
		display: flex;
	}

	/* Modal & Overlay */
	.modal-overlay, .login-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.75);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		backdrop-filter: blur(4px);
	}

	.call-modal, .login-card {
		background: #161b22;
		border: 1px solid #30363d;
		border-radius: 12px;
		padding: 28px;
		text-align: center;
		max-width: 440px;
		width: 90%;
		box-shadow: 0 10px 30px rgba(0,0,0,0.5);
	}

	.calling-avatar, .login-logo {
		font-size: 3rem;
		margin-bottom: 12px;
	}

	.call-buttons {
		display: flex;
		gap: 12px;
		justify-content: center;
		margin-top: 20px;
	}

	/* Saved Identities Section */
	.saved-identities-section {
		margin-bottom: 16px;
		text-align: left;
	}

	.saved-identities-title {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 0.75rem;
		font-weight: 600;
		color: #8b949e;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		margin-bottom: 8px;
	}

	.count-badge {
		background: #21262d;
		color: #58a6ff;
		font-size: 0.7rem;
		padding: 1px 6px;
		border-radius: 10px;
		font-weight: normal;
	}

	.saved-identities-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
		gap: 8px;
		max-height: 170px;
		overflow-y: auto;
		padding: 2px;
	}

	.saved-identity-card {
		display: flex;
		align-items: center;
		background: #0d1117;
		border: 1px solid #30363d;
		border-radius: 8px;
		padding: 6px 10px;
		transition: all 0.15s ease;
		position: relative;
	}

	.saved-identity-card:hover {
		border-color: #58a6ff;
		background: #161b22;
	}

	.saved-identity-card.active-card {
		border-color: #388bfd;
		background: #1f6feb15;
	}

	.saved-identity-action {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
		background: none;
		border: none;
		color: inherit;
		cursor: pointer;
		padding: 0;
		text-align: left;
		min-width: 0;
	}

	.saved-identity-avatar {
		font-size: 1.3rem;
	}

	.saved-identity-meta {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.saved-identity-name {
		font-size: 0.85rem;
		font-weight: 600;
		color: #f0f6fc;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.saved-identity-time {
		font-size: 0.65rem;
		color: #8b949e;
	}

	.saved-identity-del {
		background: none;
		border: none;
		color: #8b949e;
		cursor: pointer;
		font-size: 0.75rem;
		padding: 2px 4px;
		border-radius: 4px;
		margin-left: 4px;
	}

	.saved-identity-del:hover {
		background: #21262d;
		color: #f85149;
	}

	.login-divider {
		display: flex;
		align-items: center;
		text-align: center;
		margin: 16px 0;
		color: #8b949e;
		font-size: 0.75rem;
	}

	.login-divider::before,
	.login-divider::after {
		content: '';
		flex: 1;
		border-bottom: 1px solid #30363d;
	}

	.login-divider span {
		padding: 0 10px;
	}

	.remember-option {
		margin-bottom: 16px;
		text-align: left;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 0.82rem;
		color: #c9d1d9;
		cursor: pointer;
	}

	.checkbox-label input[type='checkbox'] {
		accent-color: #1f6feb;
		cursor: pointer;
		width: 15px;
		height: 15px;
	}

	.avatar-picker {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		justify-content: center;
		margin-top: 8px;
	}

	.avatar-btn {
		font-size: 1.5rem;
		background: #21262d;
		border: 2px solid transparent;
		border-radius: 8px;
		padding: 6px 10px;
		cursor: pointer;
	}

	.avatar-btn.selected {
		border-color: #58a6ff;
		background: #1f6feb33;
	}

	/* Sidebar & Workspaces */
	.chat-workspace, .dm-workspace {
		display: flex;
		width: 100%;
		height: 100%;
	}

	.sidebar {
		width: 260px;
		background: #0d1117;
		border-right: 1px solid #30363d;
		display: flex;
		flex-direction: column;
	}

	.sidebar-section {
		display: flex;
		flex-direction: column;
		border-bottom: 1px solid #21262d;
	}

	.sidebar-section.users-section {
		flex: 1;
		overflow-y: auto;
	}

	.sidebar-section-header {
		padding: 10px 14px;
		background: #161b22;
	}

	.sidebar-section-header h3 {
		margin: 0;
		font-size: 0.75rem;
		text-transform: uppercase;
		color: #8b949e;
		letter-spacing: 0.5px;
	}

	.list-container {
		padding: 8px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.list-item, .dm-user-item {
		display: flex;
		align-items: center;
		padding: 8px 10px;
		border-radius: 6px;
		background: transparent;
		border: none;
		color: #8b949e;
		cursor: pointer;
		font-size: 0.85rem;
		text-align: left;
		gap: 8px;
	}

	.list-item:hover, .dm-user-item:hover { background: #21262d; color: #f0f6fc; }
	.list-item.active, .dm-user-item.active { background: #1f6feb22; color: #58a6ff; }

	.active-dot {
		margin-left: auto;
		color: #3fb950;
		font-size: 0.6rem;
	}

	.user-row {
		display: flex;
		align-items: center;
		padding: 6px 10px;
		gap: 8px;
		border-radius: 6px;
	}

	.user-row:hover { background: #161b22; }
	.user-info { flex: 1; font-size: 0.85rem; display: flex; align-items: center; gap: 6px; }
	.user-actions { display: flex; gap: 4px; }
	.icon-btn { background: none; border: none; cursor: pointer; font-size: 1rem; padding: 2px 4px; }
	.icon-btn:hover { transform: scale(1.2); }
	.you-tag { font-size: 0.65rem; background: #238636; color: white; padding: 1px 4px; border-radius: 4px; }


	.chat-main {
		flex: 1;
		display: flex;
		flex-direction: column;
		background: #090d13;
		position: relative;
	}

	.chat-main.drag-over {
		outline: 2px dashed #58a6ff;
		outline-offset: -4px;
	}

	.drag-drop-overlay {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(13, 17, 23, 0.85);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
		pointer-events: none;
	}

	.drag-drop-card {
		background: #161b22;
		border: 2px dashed #58a6ff;
		border-radius: 12px;
		padding: 30px 40px;
		text-align: center;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
	}

	.drag-drop-icon {
		font-size: 3rem;
		display: block;
		margin-bottom: 10px;
	}

	.drag-drop-card h3 {
		margin: 0 0 6px 0;
		color: #f0f6fc;
		font-size: 1.2rem;
	}

	.drag-drop-card p {
		margin: 0;
		color: #8b949e;
		font-size: 0.9rem;
	}

	.chat-header {
		padding: 10px 16px;
		background: #161b22;
		border-bottom: 1px solid #30363d;
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.room-title {
		font-size: 1.05rem;
		font-weight: 600;
		color: #f0f6fc;
		margin-right: 8px;
	}

	.room-sub {
		font-size: 0.75rem;
		color: #8b949e;
	}

	.messages-container {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.message-bubble {
		max-width: 75%;
		padding: 10px 14px;
		border-radius: 10px;
		background: #161b22;
		border: 1px solid #30363d;
		word-break: break-word;
	}

	.message-bubble.self {
		align-self: flex-end;
		background: #1f6feb22;
		border-color: #1f6feb66;
	}

	.message-bubble.other {
		align-self: flex-start;
	}

	.bubble-meta {
		display: flex;
		gap: 8px;
		align-items: center;
		font-size: 0.75rem;
		margin-bottom: 4px;
		color: #8b949e;
	}

	.bubble-sender { font-weight: 600; color: #58a6ff; }

	.bubble-attachments {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-top: 8px;
	}

	.attachment-media {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.attachment-img {
		max-width: 320px;
		max-height: 240px;
		border-radius: 8px;
		object-fit: cover;
		display: block;
		border: 1px solid #30363d;
		transition: transform 0.15s;
		cursor: pointer;
	}

	.attachment-img:hover {
		transform: scale(1.02);
	}

	.attachment-audio {
		width: 100%;
		max-width: 320px;
		height: 36px;
	}

	.attachment-video {
		max-width: 360px;
		max-height: 240px;
		border-radius: 8px;
		background: black;
	}

	.attachment-meta {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 0.72rem;
		color: #8b949e;
	}

	.attachment-card {
		display: flex;
		align-items: center;
		gap: 10px;
		background: #0d1117;
		border: 1px solid #30363d;
		border-radius: 6px;
		padding: 8px 12px;
	}

	.attachment-file-icon {
		font-size: 1.4rem;
	}

	.attachment-card-info {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.attachment-card-info .attachment-name {
		font-size: 0.82rem;
		font-weight: 500;
		color: #c9d1d9;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.attachment-card-info .attachment-size {
		font-size: 0.7rem;
		color: #8b949e;
	}

	.btn-download {
		font-size: 0.75rem;
		padding: 4px 10px;
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		background: #21262d;
		color: #58a6ff;
		border: 1px solid #30363d;
		border-radius: 4px;
	}

	.btn-download:hover {
		background: #30363d;
		color: #79c0ff;
	}

	.chat-input-wrapper {
		background: #161b22;
		border-top: 1px solid #30363d;
		display: flex;
		flex-direction: column;
	}

	.attachment-preview-shelf {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding: 8px 16px 4px 16px;
		background: #12161c;
		border-bottom: 1px solid #21262d;
	}

	.attachment-chip {
		display: flex;
		align-items: center;
		gap: 8px;
		background: #21262d;
		border: 1px solid #30363d;
		border-radius: 6px;
		padding: 4px 8px;
		max-width: 240px;
	}

	.chip-thumb {
		width: 28px;
		height: 28px;
		object-fit: cover;
		border-radius: 4px;
	}

	.chip-icon {
		font-size: 1.1rem;
	}

	.chip-meta {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
	}

	.chip-name {
		font-size: 0.75rem;
		color: #c9d1d9;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.chip-size {
		font-size: 0.65rem;
		color: #8b949e;
	}

	.chip-remove {
		background: none;
		border: none;
		color: #8b949e;
		cursor: pointer;
		font-size: 0.8rem;
		padding: 2px 4px;
		border-radius: 4px;
	}

	.chip-remove:hover {
		background: #30363d;
		color: #f85149;
	}

	.btn-icon-attach {
		background: #21262d;
		border: 1px solid #30363d;
		color: #c9d1d9;
		font-size: 1.1rem;
		width: 36px;
		height: 36px;
		border-radius: 6px;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: background 0.15s, border-color 0.15s;
	}

	.btn-icon-attach:hover {
		background: #30363d;
		border-color: #58a6ff;
	}

	.chat-input-bar {
		padding: 10px 16px;
		display: flex;
		gap: 10px;
		align-items: center;
	}

	.chat-input-bar input {
		flex: 1;
		background: #0d1117;
		border: 1px solid #30363d;
		color: #c9d1d9;
		padding: 8px 14px;
		border-radius: 6px;
	}

	/* Wall */
	.wall-workspace {
		flex: 1;
		overflow-y: auto;
		padding: 20px;
		display: flex;
		justify-content: center;
	}

	.wall-container {
		width: 100%;
		max-width: 700px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.wall-post-card {
		padding: 16px;
	}

	.post-header {
		display: flex;
		gap: 10px;
		align-items: center;
		margin-bottom: 8px;
	}

	.post-avatar { font-size: 1.6rem; }
	.post-author-name { font-weight: 600; color: #f0f6fc; }
	.post-time { font-size: 0.75rem; color: #8b949e; }
	.post-content { font-size: 0.95rem; line-height: 1.4; color: #e6edf3; }

	/* Video Workspace */
	.video-workspace {
		flex: 1;
		padding: 20px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.video-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
	}

	.video-viewport {
		background: #000;
		border-radius: 8px;
		height: 260px;
		overflow: hidden;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.video-feed {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.video-controls {
		margin-top: 10px;
		display: flex;
		gap: 8px;
	}

	.preview-img {
		max-width: 160px;
		border-radius: 6px;
		margin-top: 6px;
	}

	.photo-gallery {
		display: flex;
		gap: 12px;
		overflow-x: auto;
		padding: 10px 0;
	}

	.photo-card img {
		width: 140px;
		height: 100px;
		object-fit: cover;
		border-radius: 6px;
	}

	/* Mini-Game */
	.game-workspace {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 20px;
	}

	.game-card {
		text-align: center;
		max-width: 420px;
		width: 100%;
	}

	.game-scoreboard {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin: 16px 0;
	}

	.score-pill {
		padding: 6px 12px;
		border-radius: 12px;
		background: #21262d;
		font-size: 0.85rem;
	}

	.score-pill.active { border: 1px solid #58a6ff; color: #58a6ff; font-weight: 600; }

	.game-board {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 8px;
		max-width: 300px;
		margin: 20px auto;
	}

	.game-cell {
		width: 90px;
		height: 90px;
		font-size: 2.2rem;
		font-weight: 700;
		background: #21262d;
		border: 1px solid #30363d;
		border-radius: 8px;
		color: #f0f6fc;
		cursor: pointer;
		transition: background 0.2s;
	}

	.game-cell:hover:not(:disabled) { background: #30363d; }
	.game-cell.X { color: #58a6ff; }
	.game-cell.O { color: #3fb950; }

	/* Form Elements & Common */
	.btn {
		background: #21262d;
		border: 1px solid #30363d;
		color: #c9d1d9;
		padding: 6px 12px;
		border-radius: 6px;
		cursor: pointer;
		font-weight: 500;
	}

	.btn:hover:not(:disabled) { background: #30363d; color: #f0f6fc; }
	.btn-primary { background: #1f6feb; border-color: #388bfd; color: white; }
	.btn-primary:hover:not(:disabled) { background: #388bfd; }
	.btn-success { background: #238636; border-color: #2ea043; color: white; }
	.btn-danger { background: #da3633; border-color: #f85149; color: white; }
	.btn-large { padding: 10px 20px; font-size: 1rem; }
	.btn-block { width: 100%; margin-top: 16px; }
	.btn-sm { padding: 4px 8px; font-size: 0.75rem; }

	.card {
		background: #161b22;
		border: 1px solid #30363d;
		border-radius: 8px;
		padding: 16px;
	}

	.input-text, .input-textarea, .input-number {
		width: 100%;
		box-sizing: border-box;
		background: #0d1117;
		border: 1px solid #30363d;
		color: #c9d1d9;
		padding: 8px 12px;
		border-radius: 6px;
	}

	.form-group {
		margin-bottom: 14px;
		text-align: left;
	}

	.form-group label {
		display: block;
		font-size: 0.8rem;
		color: #8b949e;
		margin-bottom: 6px;
	}

	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #484f58;
		font-style: italic;
	}

	.empty-hint {
		font-size: 0.75rem;
		color: #484f58;
		padding: 8px;
		text-align: center;
	}

	/* Request & Logs Tabs */
	.request-workspace {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
		padding: 20px;
		width: 100%;
		overflow-y: auto;
	}

	.preset-buttons {
		display: flex;
		gap: 6px;
		margin-bottom: 8px;
	}

	.chip {
		background: #21262d;
		border: 1px solid #30363d;
		color: #58a6ff;
		padding: 3px 8px;
		border-radius: 4px;
		font-size: 0.75rem;
		cursor: pointer;
	}

	.chip-warn { color: #d29922; }

	.code-preview {
		background: #090d13;
		padding: 12px;
		border-radius: 6px;
		font-family: monospace;
		font-size: 0.85rem;
		overflow-x: auto;
		color: #79c0ff;
	}

	.logs-workspace {
		flex: 1;
		display: flex;
		flex-direction: column;
		background: #090d13;
	}

	.logs-toolbar {
		padding: 8px 16px;
		background: #161b22;
		border-bottom: 1px solid #30363d;
		display: flex;
		justify-content: space-between;
	}

	.filter-group { display: flex; gap: 6px; }
	.filter-chip {
		background: #21262d;
		border: 1px solid #30363d;
		color: #8b949e;
		padding: 4px 10px;
		border-radius: 4px;
		font-size: 0.75rem;
		cursor: pointer;
	}
	.filter-chip.active { background: #1f6feb; color: white; }

	.logs-list {
		flex: 1;
		overflow-y: auto;
		padding: 12px;
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-family: monospace;
		font-size: 0.85rem;
	}

	.log-row {
		display: flex;
		gap: 12px;
		padding: 6px 10px;
		border-radius: 4px;
		background: #161b22;
	}

	.log-row.in { border-left: 3px solid #3fb950; }
	.log-row.out { border-left: 3px solid #58a6ff; }
	.log-row.err { border-left: 3px solid #f85149; }
	.log-row.sys { border-left: 3px solid #d29922; }

	.log-time { color: #484f58; font-size: 0.75rem; }
	.log-type-tag { font-weight: 700; color: #8b949e; width: 32px; font-size: 0.7rem; }
	.log-summary { color: #c9d1d9; flex: 1; }

	/* Floating Toast Notifications */
	.toast-notification {
		position: fixed;
		bottom: 24px;
		right: 24px;
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 18px;
		border-radius: 8px;
		background: #161b22;
		border: 1px solid #30363d;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
		color: #f0f6fc;
		font-size: 0.9rem;
		z-index: 2000;
		animation: toastSlideIn 0.25s ease-out;
	}

	@keyframes toastSlideIn {
		from {
			opacity: 0;
			transform: translateY(12px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.toast-notification.success {
		border-color: #3fb950;
	}

	.toast-notification.error {
		border-color: #f85149;
	}

	.toast-notification.info {
		border-color: #58a6ff;
	}

	.toast-icon {
		font-size: 1.1rem;
	}

	.toast-msg {
		font-weight: 500;
	}

	/* Persona Tags & Status */
	.persona-tag {
		font-size: 0.72rem;
		background: #388bfd26;
		color: #58a6ff;
		border: 1px solid #388bfd40;
		padding: 2px 7px;
		border-radius: 10px;
		font-weight: 500;
	}

	.saved-identity-status {
		font-size: 0.7rem;
		color: #58a6ff;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* Room Sidebar Items & Badges */
	.room-section-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.btn-create-room {
		background: #238636;
		color: white;
		border: none;
		border-radius: 4px;
		padding: 2px 8px;
		font-size: 0.75rem;
		cursor: pointer;
		font-weight: 600;
		transition: background 0.15s ease;
	}

	.btn-create-room:hover {
		background: #2ea043;
	}

	.pending-requests-banner {
		margin: 6px 8px 2px 8px;
		padding: 6px 10px;
		background: #d2992222;
		border: 1px solid #d2992255;
		color: #e3b341;
		border-radius: 6px;
		font-size: 0.75rem;
		font-weight: 600;
		cursor: pointer;
		text-align: left;
		transition: all 0.15s ease;
	}

	.pending-requests-banner:hover {
		background: #d2992233;
		border-color: #d2992288;
	}

	.room-icon {
		font-size: 1.05rem;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
	}

	.room-name-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex: 1;
		min-width: 0;
		gap: 6px;
	}

	.room-display-name {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		font-weight: 500;
	}

	.room-badge-protected {
		font-size: 0.65rem;
		padding: 1px 6px;
		border-radius: 10px;
		white-space: nowrap;
		font-weight: 600;
	}

	.room-badge-protected.owner {
		background: #f0883e22;
		color: #f0883e;
		border: 1px solid #f0883e44;
	}

	.room-badge-protected.member {
		background: #3fb95022;
		color: #3fb950;
		border: 1px solid #3fb95044;
	}

	.room-badge-protected.pending {
		background: #d2992222;
		color: #e3b341;
		border: 1px solid #d2992244;
	}

	.room-badge-protected.locked {
		background: #f8514922;
		color: #ff7b72;
		border: 1px solid #f8514944;
	}

	/* Chat Header Room Info */
	.chat-header-info {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}

	.room-icon-title {
		font-size: 1.3rem;
	}

	.room-header-topic {
		font-size: 0.8rem;
		color: #8b949e;
		margin-left: 4px;
	}

	.badge-warning {
		background: #d2992222;
		color: #e3b341;
		border: 1px solid #d2992255;
		font-size: 0.7rem;
		padding: 2px 7px;
		border-radius: 10px;
		font-weight: 600;
	}

	.btn-outline-warning {
		background: transparent;
		color: #e3b341;
		border: 1px solid #d2992288;
		padding: 4px 10px;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.75rem;
		font-weight: 600;
		transition: all 0.15s ease;
	}

	.btn-outline-warning:hover {
		background: #d2992222;
		border-color: #e3b341;
	}

	.chat-header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.btn-outline-recovery {
		background: #1f6feb15;
		color: #58a6ff;
		border: 1px solid #388bfd66;
		padding: 4px 12px;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.78rem;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		transition: all 0.15s ease;
	}

	.btn-outline-recovery:hover:not(:disabled) {
		background: #1f6feb33;
		border-color: #58a6ff;
		box-shadow: 0 0 8px #58a6ff44;
	}

	.btn-outline-recovery:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Modal Generic Styling */
	.modal-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
		border-bottom: 1px solid #30363d;
		padding-bottom: 10px;
	}

	.modal-header h3 {
		margin: 0;
		font-size: 1.1rem;
		color: #f0f6fc;
	}

	.modal-desc {
		font-size: 0.85rem;
		color: #8b949e;
		margin-bottom: 16px;
		text-align: left;
	}

	.btn-close-modal {
		background: none;
		border: none;
		color: #8b949e;
		font-size: 1rem;
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 4px;
	}

	.btn-close-modal:hover {
		background: #21262d;
		color: #f0f6fc;
	}

	.modal-footer {
		margin-top: 18px;
	}

	/* Owner Join Requests Modal */
	.request-modal {
		max-width: 500px;
		width: 90%;
		max-height: 80vh;
		display: flex;
		flex-direction: column;
	}

	.requests-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		overflow-y: auto;
		max-height: 380px;
	}

	.request-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 14px;
		background: #0d1117;
		border: 1px solid #30363d;
		border-radius: 8px;
		gap: 12px;
	}

	.request-avatar {
		font-size: 1.5rem;
	}

	.request-user-info {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1;
		min-width: 0;
		text-align: left;
	}

	.request-name {
		font-weight: 600;
		font-size: 0.9rem;
		color: #f0f6fc;
	}

	.request-status-tag {
		font-size: 0.7rem;
		color: #58a6ff;
	}

	.request-room {
		font-size: 0.75rem;
		color: #8b949e;
	}

	.request-time {
		font-size: 0.68rem;
		color: #484f58;
	}

	.request-actions {
		display: flex;
		gap: 8px;
	}

	/* Permission Prompt Modal */
	.permission-modal {
		max-width: 440px;
		width: 90%;
		text-align: center;
	}

	.permission-header {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		margin-bottom: 12px;
	}

	.permission-icon {
		font-size: 2.8rem;
	}

	.permission-room-badge {
		font-size: 0.75rem;
		color: #ff7b72;
		background: #f8514922;
		border: 1px solid #f8514944;
		padding: 2px 8px;
		border-radius: 12px;
		font-weight: 600;
	}

	.permission-topic {
		font-size: 0.85rem;
		color: #8b949e;
		margin: 8px 0;
	}

	.permission-owner-tag {
		font-size: 0.75rem;
		color: #e3b341;
		margin-bottom: 12px;
	}

	.permission-status-box {
		margin: 14px 0;
	}

	.pending-notice {
		background: #d2992218;
		border: 1px solid #d2992244;
		border-radius: 8px;
		padding: 12px;
		color: #e3b341;
		font-size: 0.85rem;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.denied-notice {
		background: #f8514918;
		border: 1px solid #f8514944;
		border-radius: 8px;
		padding: 12px;
		color: #ff7b72;
		font-size: 0.85rem;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	/* Create Room Modal */
	.create-room-modal {
		max-width: 480px;
		width: 90%;
		text-align: left;
	}

	.icon-picker {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-top: 6px;
	}

	.icon-picker-btn {
		font-size: 1.3rem;
		background: #21262d;
		border: 2px solid transparent;
		border-radius: 8px;
		padding: 6px 10px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.icon-picker-btn:hover {
		background: #30363d;
	}

	.icon-picker-btn.selected {
		border-color: #58a6ff;
		background: #1f6feb33;
	}

	.privacy-selector {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-top: 8px;
	}

	.privacy-option {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		background: #0d1117;
		border: 1px solid #30363d;
		border-radius: 8px;
		padding: 12px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.privacy-option:hover {
		border-color: #58a6ff;
		background: #161b22;
	}

	.privacy-option.selected {
		border-color: #388bfd;
		background: #1f6feb15;
	}

	.privacy-option input[type='radio'] {
		accent-color: #1f6feb;
		margin-top: 3px;
		cursor: pointer;
	}

	.privacy-details {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}

	.privacy-title {
		font-weight: 600;
		font-size: 0.9rem;
		color: #f0f6fc;
	}

	.privacy-desc {
		font-size: 0.78rem;
		color: #8b949e;
	}

	/* Message Recall (Unsend) Styles */
	.btn-recall-msg {
		background: transparent;
		border: 1px solid #30363d;
		color: #8b949e;
		font-size: 0.65rem;
		padding: 1px 6px;
		border-radius: 4px;
		cursor: pointer;
		margin-left: auto;
		opacity: 0.75;
		transition: all 0.15s ease;
	}

	.btn-recall-msg:hover {
		opacity: 1;
		color: #f85149;
		border-color: #f8514988;
		background: #f8514915;
	}

	.message-bubble.recalled {
		opacity: 0.7;
		background: #161b2280 !important;
		border: 1px dashed #30363d !important;
	}

	.recalled-content {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #8b949e;
		font-style: italic;
		font-size: 0.85rem;
		padding: 4px 0;
	}

	.recalled-icon {
		font-size: 0.85rem;
	}

	/* Facebook-Style Read Receipts (Seen indicators) */
	.read-receipts-row {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 6px;
		margin-top: 6px;
		padding-top: 2px;
	}

	.seen-label {
		font-size: 0.68rem;
		color: #8b949e;
		font-weight: 500;
		letter-spacing: 0.3px;
	}

	.seen-avatars {
		display: flex;
		align-items: center;
		flex-direction: row-reverse;
	}

	.seen-avatar-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background: #21262d;
		border: 1.5px solid #0d1117;
		font-size: 11px;
		margin-left: -5px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
		transition: transform 0.15s ease, z-index 0.15s ease;
		cursor: default;
	}

	.seen-avatar-badge:hover {
		transform: scale(1.4);
		z-index: 10;
		border-color: #58a6ff;
	}

	/* DM Read Receipts */
	.dm-status-row {
		display: flex;
		justify-content: flex-end;
		margin-top: 4px;
		font-size: 0.68rem;
	}

	.dm-read-status {
		color: #58a6ff;
		font-weight: 500;
	}

	.dm-sent-status {
		color: #8b949e;
	}

	/* Unread Badge on Nav Tabs */
	.tab-badge {
		background: #f85149;
		color: white;
		font-size: 0.65rem;
		font-weight: 700;
		padding: 1px 6px;
		border-radius: 10px;
		margin-left: 6px;
		box-shadow: 0 0 8px rgba(248, 81, 73, 0.4);
	}

	/* User Row & Room Unread Badges in Sidebar & DM list */
	.badge-unread-user,
	.badge-unread-room,
	.dm-unread-badge {
		background: #f85149;
		color: white;
		font-size: 0.65rem;
		font-weight: 700;
		padding: 2px 6px;
		border-radius: 10px;
		margin-right: 4px;
		box-shadow: 0 0 6px rgba(248, 81, 73, 0.35);
	}

	/* Live Public Wall & Broadcast Banner (Unified Workspace) */
	.public-wall-banner {
		background: linear-gradient(135deg, rgba(31, 111, 235, 0.12), rgba(130, 80, 223, 0.12));
		border-bottom: 1px solid rgba(88, 166, 255, 0.25);
		padding: 10px 16px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.wall-banner-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
	}

	.wall-banner-title {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 0.85rem;
	}

	.wall-pulse {
		color: #3fb950;
		font-size: 0.75rem;
		animation: pulse-dot 1.6s infinite;
	}

	@keyframes pulse-dot {
		0%, 100% { opacity: 1; transform: scale(1); }
		50% { opacity: 0.4; transform: scale(0.85); }
	}

	.wall-title-text {
		font-weight: 600;
		color: #58a6ff;
	}

	.wall-latest-preview {
		color: #c9d1d9;
		font-size: 0.82rem;
		max-width: 420px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		background: rgba(13, 17, 23, 0.6);
		padding: 2px 8px;
		border-radius: 6px;
		border: 1px solid #30363d;
	}

	.wall-banner-controls {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.btn-outline-banner {
		background: #21262d;
		border: 1px solid #388bfd44;
		color: #58a6ff;
		padding: 2px 8px;
		font-size: 0.75rem;
		border-radius: 4px;
		cursor: pointer;
	}

	.btn-outline-banner:hover {
		background: #1f6feb22;
		border-color: #58a6ff;
	}

	.btn-wall-link {
		background: transparent;
		border: none;
		color: #8b949e;
		font-size: 0.75rem;
		cursor: pointer;
		text-decoration: underline;
	}

	.btn-wall-link:hover {
		color: #58a6ff;
	}

	.wall-quick-composer {
		display: flex;
		gap: 8px;
	}

	.wall-quick-composer input {
		flex: 1;
		background: #0d1117;
		border: 1px solid #30363d;
		border-radius: 6px;
		padding: 6px 12px;
		color: #c9d1d9;
		font-size: 0.85rem;
	}

	.wall-quick-composer input:focus {
		outline: none;
		border-color: #58a6ff;
	}

	/* DM Quick Peers Grid in Empty State */
	.dm-empty-picker {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 40px 20px;
		text-align: center;
	}

	.dm-quick-peers-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 12px;
		width: 100%;
		max-width: 600px;
	}

	.dm-quick-peer-card {
		background: #161b22;
		border: 1px solid #30363d;
		border-radius: 8px;
		padding: 14px;
		display: flex;
		flex-direction: column;
		align-items: center;
		transition: transform 0.15s ease, border-color 0.15s ease;
	}

	.dm-quick-peer-card:hover {
		transform: translateY(-2px);
		border-color: #58a6ff;
	}

	/* Video Peer Call Launcher */
	.video-peer-launcher {
		background: #161b22;
		border: 1px solid #30363d;
		border-radius: 8px;
		padding: 12px 16px;
		margin-bottom: 16px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.launcher-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 0.85rem;
	}

	.launcher-title {
		font-weight: 600;
		color: #58a6ff;
	}

	.sub-hint {
		font-size: 0.75rem;
		color: #8b949e;
	}

	.launcher-chips {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.peer-call-chip {
		background: #21262d;
		border: 1px solid #30363d;
		border-radius: 20px;
		padding: 4px 12px;
		color: #f0f6fc;
		display: flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		font-size: 0.82rem;
		transition: all 0.15s ease;
	}

	.peer-call-chip:hover {
		background: #23863622;
		border-color: #238636;
		color: #3fb950;
	}

	.peer-call-chip.calling-chip {
		background: #23863644;
		border-color: #3fb950;
		color: #3fb950;
		box-shadow: 0 0 8px rgba(63, 185, 80, 0.4);
	}

	/* Mini-Game AI Opponent Thinking & Footer */
	.ai-thinking-pill {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		background: rgba(88, 166, 255, 0.15);
		border: 1px solid rgba(88, 166, 255, 0.3);
		color: #58a6ff;
		padding: 2px 8px;
		border-radius: 12px;
		font-size: 0.78rem;
		animation: pulse-dot 1.2s infinite;
	}

	.game-auto-reply-note {
		font-size: 0.75rem;
		color: #8b949e;
		margin-left: 12px;
	}
</style>
