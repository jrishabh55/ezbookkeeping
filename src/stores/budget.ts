import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import {
    type BudgetProgressResponse,
    type BudgetCreateRequest,
    type BudgetModifyRequest,
    type BudgetNewDisplayOrderRequest,
    Budget
} from '@/models/budget.ts';

import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useBudgetsStore = defineStore('budgets', () => {

    const allBudgets = ref<Budget[]>([]);
    const allBudgetsMap = ref<Record<string, Budget>>({});
    const budgetListStateInvalid = ref<boolean>(true);

    const allVisibleBudgets = computed<Budget[]>(() => {
        return allBudgets.value.filter(budget => !budget.hidden);
    });

    const allAvailableBudgetsCount = computed<number>(() => {
        return allBudgets.value.length;
    });

    const allVisibleBudgetsCount = computed<number>(() => {
        return allVisibleBudgets.value.length;
    });

    function loadBudgetList(budgets: Budget[]): void {
        allBudgets.value = budgets;
        allBudgetsMap.value = {};

        for (const budget of budgets) {
            allBudgetsMap.value[budget.id] = budget;
        }
    }

    function addBudgetToBudgetList(budget: Budget): void {
        allBudgets.value.push(budget);
        allBudgetsMap.value[budget.id] = budget;
    }

    function updateBudgetInBudgetList(budget: Budget): void {
        for (let i = 0; i < allBudgets.value.length; i++) {
            if (allBudgets.value[i]?.id === budget.id) {
                allBudgets.value[i] = budget;
                break;
            }
        }
        allBudgetsMap.value[budget.id] = budget;
    }

    function removeBudgetFromBudgetList(budgetId: string): void {
        for (let i = 0; i < allBudgets.value.length; i++) {
            if (allBudgets.value[i]?.id === budgetId) {
                allBudgets.value.splice(i, 1);
                break;
            }
        }
        delete allBudgetsMap.value[budgetId];
    }

    function updateBudgetDisplayOrderInBudgetList(budgets: Budget[]): void {
        for (const budget of budgets) {
            const existingBudget = allBudgetsMap.value[budget.id];
            if (existingBudget) {
                existingBudget.displayOrder = budget.displayOrder;
            }
        }

        allBudgets.value.sort((a, b) => a.displayOrder - b.displayOrder);
    }

    function updateBudgetVisibilityInBudgetList(budgetId: string, hidden: boolean): void {
        const budget = allBudgetsMap.value[budgetId];
        if (budget) {
            budget.hidden = hidden;
        }
    }

    function resetBudgets(): void {
        allBudgets.value = [];
        allBudgetsMap.value = {};
        budgetListStateInvalid.value = true;
    }

    function loadAllBudgets({ force }: { force: boolean }): Promise<Budget[]> {
        return new Promise((resolve, reject) => {
            if (!force && !budgetListStateInvalid.value) {
                resolve(allBudgets.value);
                return;
            }

            services.getAllBudgets().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve budget list' });
                    return;
                }

                const budgets: Budget[] = [];

                for (const budgetResp of data.result) {
                    budgets.push(Budget.fromResponse(budgetResp));
                }

                loadBudgetList(budgets);
                budgetListStateInvalid.value = false;

                resolve(budgets);
            }).catch(error => {
                logger.error('failed to load all budgets', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve budget list' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function getBudget({ budgetId }: { budgetId: string }): Promise<Budget> {
        return new Promise((resolve, reject) => {
            services.getBudget({ id: budgetId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve budget' });
                    return;
                }

                const budget = Budget.fromResponse(data.result);
                resolve(budget);
            }).catch(error => {
                logger.error('failed to get budget', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve budget' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function saveBudget({ budget, isEdit }: { budget: BudgetCreateRequest | BudgetModifyRequest, isEdit: boolean }): Promise<Budget> {
        return new Promise((resolve, reject) => {
            let promise;

            if (!isEdit) {
                promise = services.addBudget(budget as BudgetCreateRequest);
            } else {
                promise = services.modifyBudget(budget as BudgetModifyRequest);
            }

            promise.then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save budget' });
                    return;
                }

                const savedBudget = Budget.fromResponse(data.result);

                if (!isEdit) {
                    addBudgetToBudgetList(savedBudget);
                } else {
                    updateBudgetInBudgetList(savedBudget);
                }

                resolve(savedBudget);
            }).catch(error => {
                logger.error('failed to save budget', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to save budget' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function changeBudgetDisplayOrder({ newDisplayOrders }: { newDisplayOrders: BudgetNewDisplayOrderRequest[] }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.moveBudget({ newDisplayOrders }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to move budget' });
                    return;
                }

                const budgets: Budget[] = [];

                for (const order of newDisplayOrders) {
                    budgets.push(new Budget(
                        order.id,
                        '',
                        0,
                        '',
                        0,
                        0,
                        0,
                        0,
                        false,
                        0,
                        0,
                        0,
                        order.displayOrder,
                        false
                    ));
                }

                updateBudgetDisplayOrderInBudgetList(budgets);

                resolve(true);
            }).catch(error => {
                logger.error('failed to move budget', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to move budget' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function hideBudget({ budget, hidden }: { budget: Budget, hidden: boolean }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.hideBudget({ id: budget.id, hidden }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to hide budget' });
                    return;
                }

                updateBudgetVisibilityInBudgetList(budget.id, hidden);

                resolve(true);
            }).catch(error => {
                logger.error('failed to hide budget', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to hide budget' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteBudget({ budget }: { budget: Budget }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteBudget({ id: budget.id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete budget' });
                    return;
                }

                removeBudgetFromBudgetList(budget.id);

                resolve(true);
            }).catch(error => {
                logger.error('failed to delete budget', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete budget' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function getBudgetProgress({ budgetId, startTime, endTime }: { budgetId: string, startTime?: number, endTime?: number }): Promise<BudgetProgressResponse> {
        return new Promise((resolve, reject) => {
            services.getBudgetProgress({ id: budgetId, startTime, endTime }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve budget progress' });
                    return;
                }

                resolve(data.result);
            }).catch(error => {
                logger.error('failed to get budget progress', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve budget progress' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        allBudgets,
        allBudgetsMap,
        budgetListStateInvalid,
        allVisibleBudgets,
        allAvailableBudgetsCount,
        allVisibleBudgetsCount,
        resetBudgets,
        loadAllBudgets,
        getBudget,
        saveBudget,
        changeBudgetDisplayOrder,
        hideBudget,
        deleteBudget,
        getBudgetProgress
    };
});
