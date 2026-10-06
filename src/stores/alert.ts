import { defineStore } from 'pinia';

import type {
    AlertTokenCreateResponse,
    AlertStatusResponse
} from '@/models/alert.ts';

import logger from '@/lib/logger.ts';
import services from '@/lib/services.ts';

export const useAlertsStore = defineStore('alerts', () => {
    function createAlertToken({ password }: { password: string }): Promise<AlertTokenCreateResponse> {
        return new Promise((resolve, reject) => {
            services.createAlertToken({ password }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to set up SMS Auto-capture' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to create alert ingest token', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to set up SMS Auto-capture' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function revokeAlertToken(): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.revokeAlertToken().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to revoke SMS Auto-capture' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to revoke alert ingest token', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to revoke SMS Auto-capture' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function getAlertStatus(): Promise<AlertStatusResponse> {
        return new Promise((resolve, reject) => {
            services.getAlertStatus().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve SMS Auto-capture status' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load alert ingest status', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve SMS Auto-capture status' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        // functions
        createAlertToken,
        revokeAlertToken,
        getAlertStatus
    };
});
